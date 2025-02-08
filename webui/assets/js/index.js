if (window.location.pathname === "/home" || window.location.pathname === "/") {
    // WebSocket-Verbindung initialisieren
    // const wsIndex = new WebSocket(`${WS_PATH}/dashboardData`);
    var hostname = window.location.hostname;
    const wsIndex = new WebSocket(`wss://${hostname}/api/dashboardData`);

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
}
