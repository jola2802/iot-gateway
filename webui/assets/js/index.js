(async function init() {
    if (window.location.pathname === "/home" || window.location.pathname === "/") {
        var hostname = window.location.hostname;

        // Zuerst den Token vom Server abrufen
        await fetch(`/api/ws-token`)
            .then(response => {
                if (!response.ok) {
                    throw new Error("Token-Abruf fehlgeschlagen", response);
                }
                return response.json();
            })
            .then(data => {
                // Erwartet: { token: "abcdef...", expires: "..." }
                const token = data.token;
                if (!token) {
                    throw new Error("Kein Token erhalten");
                }
                // WebSocket-URL mit dem Token als Query-Parameter aufbauen
                const wsUrl = `/api/ws-broker-status?token=${encodeURIComponent(token)}`;
                const wsIndex = new WebSocket(wsUrl);

                // Funktion zum Aktualisieren der Dashboard-Daten
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

                        // Node-RED-URL aktualisieren nur wenn der Wert nicht leer ist
                        if (data.nodeRedURL !== undefined) {
                            fetch('/api/get-node-red-url')
                                .then(response => response.json())
                            .then(data => {
                                nodeRedElement.href = data.nodeRedURL;
                            });
                        }
                    }
                    // Node-RED-Adresse aktualisieren wenn noch kein wert vorhanden ist
                    const nodeRedAddressElement = document.getElementById('node-red-address');
                    
                    // Check ob bereits ein wert in nodeRedAddressElement gespeichert ist
                    if (nodeRedAddressElement.href === undefined || nodeRedAddressElement.href === "" || nodeRedAddressElement.href === null) {
                        // Node-RED-URL aktualisieren nur wenn der Wert nicht leer ist
                        fetch('/api/get-node-red-url')
                        .then(response => response.json())
                        .then(data => {
                            nodeRedAddressElement.href = data.nodeRedURL;
                            nodeRedAddressElement.style.color = 'black';
                        });
                    }
                }

                // WebSocket-Ereignisse
                wsIndex.onopen = () => {
                    console.log('WebSocket-Verbindung hergestellt');
                };

                wsIndex.onmessage = (event) => {
                    try {
                        const data = JSON.parse(event.data);
                        updateDashboardData(data);
                    } catch (error) {
                        console.error('Fehler beim Verarbeiten der WebSocket-Daten:', error);
                    }
                };

                wsIndex.onerror = (error) => {
                    console.error('WebSocket-Fehler:', error);
                };

                wsIndex.onclose = (event) => {
                    console.warn('WebSocket-Verbindung geschlossen:', event.reason);
                };
            })
            .catch(error => {
                console.error('Fehler beim Abrufen des Tokens:', error);
            });
    }
})();