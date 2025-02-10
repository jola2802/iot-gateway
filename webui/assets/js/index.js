(async function init() {
    if (window.location.pathname === "/home" || window.location.pathname === "/") {
        var hostname = window.location.hostname;

        // Zuerst den Token vom Server abrufen
        await fetch(`/ws-token`)
            .then(response => {
                if (!response.ok) {
                    console.log('response:', response);
                    throw new Error("Token-Abruf fehlgeschlagen", response);
                }
                console.log('response:', response);
                return response.json();
            })
            .then(data => {
                // Erwartet: { token: "abcdef...", expires: "..." }
                const token = data.token;
                if (!token) {
                    throw new Error("Kein Token erhalten");
                }
                // WebSocket-URL mit dem Token als Query-Parameter aufbauen
                const wsUrl = `/ws-broker-status?token=${encodeURIComponent(token)}`;
                const wsIndex = new WebSocket(wsUrl);

                console.log('received token:', token);

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
                        console.log('WebSocket-Daten empfangen:', data);
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