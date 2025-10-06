(async function init() {
    if (window.location.pathname === "/home" || window.location.pathname === "/") {
        var hostname = window.location.hostname;

        // First get the token from the server
        await fetch(`/api/ws-token`)
            .then(response => {
                if (!response.ok) {
                    throw new Error("Token-Call failed", response);
                }
                return response.json();
            })
            .then(data => {
                const token = data.token;
                if (!token) {
                    throw new Error("No token received");
                }
                // Build the WebSocket URL with the token as a query parameter
                const wsUrl = `/api/ws-broker-status?token=${encodeURIComponent(token)}`;
                const wsIndex = new WebSocket(wsUrl);

                // Function to update the dashboard data
                function updateDashboardData(data) {
                    if (data.uptime !== undefined) {
                        document.getElementById('uptime').textContent = data.uptime;
                    }
                    if (data.numberDevices !== undefined) {
                        document.getElementById('number-devices').textContent = data.numberDevices;
                    }
                    if (data.numberMessages !== undefined) {
                        document.getElementById('number-messages').textContent = data.numberMessages;
                    }
                    if (data.nodeRedConnection !== undefined) {
                        const nodeRedElement = document.getElementById('node-red-connection');
                        nodeRedElement.textContent = data.nodeRedConnection ? 'Connected' : 'Disconnected';
                        nodeRedElement.style.color = data.nodeRedConnection ? 'green' : 'red';

                    }
                    // Update the Node-RED address if there is no value yet
                    const nodeRedAddressElement = document.getElementById('node-red-address');
                    
                    // Check if there is already a value in nodeRedAddressElement
                    if (nodeRedAddressElement.href === undefined || nodeRedAddressElement.href === "" || nodeRedAddressElement.href === null) {
                        // Extrahiere IP-Adresse aus der aktuellen URL
                        let nodeRedHost = "127.0.0.1"; // Fallback
                        
                        if (hostname && hostname !== "localhost") {
                            nodeRedHost = hostname;
                        }
                        
                        nodeRedAddressElement.style.color = 'black';
                        nodeRedAddressElement.href = `http://${nodeRedHost}:1880`;
                    }
                }

                // WebSocket events
                wsIndex.onopen = () => {
                    console.log('WebSocket connection established');
                };

                wsIndex.onmessage = (event) => {
                    try {
                        const data = JSON.parse(event.data);
                        updateDashboardData(data);
                    } catch (error) {
                        console.error('Error processing WebSocket data:', error);
                    }
                };

                wsIndex.onerror = (error) => {
                    console.error('WebSocket error:', error);
                };

                wsIndex.onclose = (event) => {
                    console.warn('WebSocket connection closed:', event.reason);
                };
            })
            .catch(error => {
                console.error('Error fetching token:', error);
            });
    }
})();