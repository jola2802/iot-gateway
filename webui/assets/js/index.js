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
                    }
                    // Node-RED-Adresse aktualisieren
                    if (data.nodeRedAddress !== undefined) {
                        const nodeRedAddressElement = document.getElementById('node-red-address');
                        nodeRedAddressElement.textContent = data.nodeRedAddress;
                        nodeRedAddressElement.href = data.nodeRedAddress;
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

// eventlistener for the confirm-restart-button
document.getElementById('confirm-restart-button').addEventListener('click', restartGateway);

// Function to restart the gateway
async function restartGateway() {
    await fetch('/api/restart', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Fehler beim Neustarten des Gateways');
        } else {
            alert('Gateway wurde erfolgreich neugestartet');
            window.location.reload();
        }
    })
}