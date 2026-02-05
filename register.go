package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/kr/pty"
	"github.com/mitchellh/colorstring"
	"github.com/pion/webrtc/v3"
)

type registerSession struct {
	session
	id        string
	host      string
	ptmx      *os.File
	ptmxReady bool
}

func (rs *registerSession) run() error {
	if err := rs.init(); err != nil {
		return err
	}

	colorstring.Printf("[bold]Creating registration offer for ID: %s\n", rs.id)

	if err := rs.createOffer(); err != nil {
		return err
	}

	rs.pc.OnDataChannel(rs.onDataChannel())

	reqBody := RegisterRequest{
		ID:  rs.id,
		SDP: rs.offer,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := rs.sendRegistrationWithRetry(jsonData); err != nil {
		return err
	}

	colorstring.Printf("[bold]Waiting for connection (60 seconds)...\n")
	answerSDP, err := rs.pollForAnswer()
	if err != nil {
		return err
	}

	colorstring.Printf("[bold][green]Answer received! Connecting...\n")
	return rs.setRemoteDescriptionAndWait(answerSDP)
}

func (rs *registerSession) createOffer() error {
	if _, err := rs.pc.CreateDataChannel("offerer-channel", nil); err != nil {
		log.Println(err)
		return err
	}

	offer, err := rs.pc.CreateOffer(nil)
	if err != nil {
		log.Println(err)
		return err
	}

	gatherComplete := webrtc.GatheringCompletePromise(rs.pc)

	err = rs.pc.SetLocalDescription(offer)
	if err != nil {
		log.Println(err)
		return err
	}

	<-gatherComplete

	rs.offer = rs.pc.LocalDescription().SDP
	return nil
}

func (rs *registerSession) sendRegistrationWithRetry(jsonData []byte) error {
	const maxRetries = 3
	const baseDelay = 1 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			colorstring.Printf("[yellow]Retrying in %v... (attempt %d/%d)\n",
				delay, attempt+1, maxRetries)
			time.Sleep(delay)
		}

		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Post(
			rs.host+"/api/1/register",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			lastErr = fmt.Errorf("network error: %w", err)
			continue
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated:
			colorstring.Printf("[bold][green]Successfully registered with ID: %s\n", rs.id)
			colorstring.Printf("[bold]Registration URL: %s/api/1/connect/%s\n", rs.host, rs.id)
			return nil

		case http.StatusConflict:
			return fmt.Errorf("ID '%s' already registered. Please choose a different ID", rs.id)

		case http.StatusBadRequest:
			var errResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errResp)
			return fmt.Errorf("invalid request: %s", errResp["error"])

		default:
			lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
			continue
		}
	}

	return fmt.Errorf("registration failed after %d attempts: %w", maxRetries, lastErr)
}

func (rs *registerSession) pollForAnswer() (string, error) {
	const pollInterval = 2 * time.Second
	const maxWait = 60 * time.Second
	deadline := time.Now().Add(maxWait)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for time.Now().Before(deadline) {
		resp, err := client.Get(rs.host + "/api/1/answer/" + rs.id)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			time.Sleep(pollInterval)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var result struct {
				ID     string `json:"id"`
				Answer string `json:"answer"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return "", fmt.Errorf("failed to decode answer: %w", err)
			}
			return result.Answer, nil
		}

		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("server error: %s", errResp["error"])
	}

	return "", fmt.Errorf("timeout waiting for connection (60 seconds)")
}

func (rs *registerSession) setRemoteDescriptionAndWait(answerSDP string) error {
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerSDP,
	}

	if err := rs.pc.SetRemoteDescription(answer); err != nil {
		log.Println(err)
		return err
	}

	colorstring.Printf("[bold][green]Terminal session connected!\n")
	err := <-rs.errChan
	rs.cleanup()
	if rs.ptmx != nil {
		rs.ptmx.Close()
	}
	return err
}

func (rs *registerSession) onDataChannel() func(dc *webrtc.DataChannel) {
	return func(dc *webrtc.DataChannel) {
		rs.dc = dc
		dc.OnOpen(rs.dataChannelOnOpen())
		dc.OnMessage(rs.dataChannelOnMessage())
	}
}

func (rs *registerSession) dataChannelOnOpen() func() {
	return func() {
		colorstring.Println("[bold][green]Data channel opened - Starting terminal...")

		cmd := exec.Command("bash", "-l")
		var err error
		rs.ptmx, err = pty.Start(cmd)
		if err != nil {
			log.Println(err)
			rs.errChan <- err
			return
		}
		rs.ptmxReady = true

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			for range c {
				log.Println("Sigint")
				rs.errChan <- errors.New("sigint")
			}
		}()

		buf := make([]byte, 1024)
		for {
			nr, err := rs.ptmx.Read(buf)
			if err != nil {
				if err == io.EOF {
					err = nil
				} else {
					log.Println(err)
				}
				rs.errChan <- err
				return
			}
			if err = rs.dc.Send(buf[0:nr]); err != nil {
				log.Println(err)
				rs.errChan <- err
				return
			}
		}
	}
}

func (rs *registerSession) dataChannelOnMessage() func(payload webrtc.DataChannelMessage) {
	return func(p webrtc.DataChannelMessage) {
		for !rs.ptmxReady {
			time.Sleep(1 * time.Millisecond)
		}

		if p.IsString {
			if len(p.Data) > 2 && p.Data[0] == '[' && p.Data[1] == '"' {
				var msg []string
				err := json.Unmarshal(p.Data, &msg)
				if len(msg) == 0 {
					log.Println(err)
					rs.errChan <- err
					return
				}
				if msg[0] == "stdin" {
					toWrite := []byte(msg[1])
					if len(toWrite) == 0 {
						return
					}
					_, err := rs.ptmx.Write(toWrite)
					if err != nil {
						log.Println(err)
						rs.errChan <- err
					}
					return
				}
				if msg[0] == "set_size" {
					var size []int
					_ = json.Unmarshal(p.Data, &size)
					if len(size) >= 3 {
						ws, err := pty.GetsizeFull(rs.ptmx)
						if err != nil {
							log.Println(err)
							return
						}
						ws.Rows = uint16(size[1])
						ws.Cols = uint16(size[2])
						if len(size) >= 5 {
							ws.X = uint16(size[3])
							ws.Y = uint16(size[4])
						}
						if err := pty.Setsize(rs.ptmx, ws); err != nil {
							log.Println(err)
						}
					}
					return
				}
			}
			if string(p.Data) == "quit" {
				rs.errChan <- nil
				return
			}
		} else {
			_, err := rs.ptmx.Write(p.Data)
			if err != nil {
				log.Println(err)
				rs.errChan <- err
			}
		}
	}
}
