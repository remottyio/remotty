let term = null;
let fitAddon = null;
let sendChannel = null;

async function showSDP(hostId) {
    const terminalModal = document.getElementById('terminalModal');
    const terminalTitle = document.getElementById('terminalTitle');

    terminalModal.style.display = 'block';
    terminalTitle.textContent = 'Connecting to: ' + hostId;

    // Create terminal
    term = new Terminal({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        theme: {
            background: '#000000',
            foreground: '#ffffff'
        }
    });

    fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    term.open(document.getElementById('terminal'));
    fitAddon.fit();

    window.addEventListener('resize', () => {
        if (fitAddon) fitAddon.fit();
    });

    term.writeln('Connecting to ' + hostId + '...');

    try {
        // Fetch SDP offer
        const response = await fetch('/api/1/connect/' + hostId);
        const data = await response.json();

        if (!response.ok) {
            term.writeln('\r\nError: ' + (data.error || 'Failed to fetch SDP'));
            return;
        }

        term.writeln('Setting up WebRTC connection...');

        // Create WebRTC peer connection
        const pc = new RTCPeerConnection({
            iceServers: [{
                urls: 'stun:stun.l.google.com:19302'
            }]
        });

        // Connection state handlers
        pc.onconnectionstatechange = () => {
            term.writeln('\r\nConnection state: ' + pc.connectionState);
        };

        pc.oniceconnectionstatechange = () => {
            term.writeln('\r\nICE connection state: ' + pc.iceConnectionState);
        };

        // Create data channel
        sendChannel = pc.createDataChannel('data');
        sendChannel.binaryType = 'arraybuffer';

        term.writeln('Data channel created...');

        sendChannel.onopen = () => {
            term.clear();
            term.writeln('');
            term.writeln('Data channel OPEN! Terminal ready.');
            term.writeln('');
            terminalTitle.textContent = 'Connected to: ' + hostId;

            // Send terminal size
            sendChannel.send(JSON.stringify(['set_size', term.rows, term.cols]));
        };

        sendChannel.onclose = () => {
            term.writeln('\r\n\r\nConnection closed.');
            terminalTitle.textContent = 'Disconnected';
        };

        sendChannel.onmessage = (event) => {
            if (typeof event.data === 'string') {
                term.write(event.data);
            } else if (event.data instanceof ArrayBuffer) {
                const data = new Uint8Array(event.data);
                term.write(data);
            }
        };

        // Handle terminal input
        term.onData((data) => {
            if (sendChannel && sendChannel.readyState === 'open') {
                sendChannel.send(JSON.stringify(['stdin', data]));
            }
        });

        // Set remote description (the offer from host)
        await pc.setRemoteDescription({
            type: 'offer',
            sdp: data.sdp
        });

        term.writeln('Creating answer...');

        // Create answer
        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);

        // Wait for ICE gathering to complete
        await new Promise((resolve) => {
            if (pc.iceGatheringState === 'complete') {
                resolve();
            } else {
                pc.onicegatheringstatechange = () => {
                    if (pc.iceGatheringState === 'complete') {
                        resolve();
                    }
                };
            }
        });

        term.writeln('Sending answer to host...');

        // Send answer to server
        const answerResponse = await fetch('/api/1/answer/' + hostId, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                answer: pc.localDescription.sdp
            })
        });

        if (!answerResponse.ok) {
            const errorData = await answerResponse.json();
            term.writeln('\r\nError: ' + (errorData.error || 'Failed to send answer'));
            return;
        }

        term.writeln('Waiting for host...');

    } catch (error) {
        term.writeln('\r\nError: ' + error.message);
    }
}

function closeTerminal() {
    if (sendChannel) {
        sendChannel.close();
    }
    if (term) {
        term.dispose();
        term = null;
    }
    document.getElementById('terminalModal').style.display = 'none';
}
