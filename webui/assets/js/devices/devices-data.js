let wsDevices;

// Funktion zum Initialisieren der WebSocket-Verbindung
function initWebSocket(WS_PATH) {
    wsDevices = new WebSocket(`${WS_PATH}/deviceData`);

    wsDevices.onopen = () => {
        console.log('WebSocket-Verbindung hergestellt');
    };

    wsDevices.onmessage = (event) => {
        const devices = JSON.parse(event.data);
        updateDeviceTables(devices);
    };

    wsDevices.onerror = (error) => {
        console.error('WebSocket-Fehler:', error);
    };

    wsDevices.onclose = (event) => {
        console.warn('WebSocket-Verbindung geschlossen:', event.reason);
    };
}

// Funktion zum Speichern der Gerätekonfiguration bei Edit Device Modal
async function saveDevice() {
    try {
        // Gerätedaten aus dem Modal abrufen
        const deviceId = localStorage.getItem('device_id');
        const deviceName = document.getElementById('device-name-1').value;
        const deviceType = document.getElementById('select-device-type-1').value;

        // Allgemeine Konfiguration je nach Typ abrufen
        let config = {};
        if (deviceType === 'opc-ua') {
            config = {
                address: document.getElementById('address-1').value,
                securityPolicy: document.getElementById('select-security-policy-1').value,
                authentication: document.getElementById('select-authentication-settings-1').value,
                securityMode: document.getElementById('select-security-mode-1').value,
                username: document.getElementById('username')?.value || null,
                password: document.getElementById('password')?.value || null,
            };
        } else if (deviceType === 's7') {
            config = {
                address: document.querySelector('#s7-config-1 [placeholder="192.168.2.100:102"]').value,
                rack: document.querySelector('#s7-config-1 [placeholder="0"]').value,
                slot: document.querySelector('#s7-config-1 [placeholder="1"]').value,
                acquisitionTime: document.getElementById('aquisition-time-2').value,
            };
        } else if (deviceType === 'mqtt') {
            config = {
                password: document.querySelector('#mqtt-config-1 [placeholder="Type in password"]').value,
            };
        }

        // Datapoints abrufen und leere Zeile überspringen
        const datapoints = Array.from(document.querySelectorAll('#ipi-table tbody tr'))
            .map((row) => {
                const cells = row.querySelectorAll('td');
                const id = cells[0]?.textContent.trim();
                const name = cells[1]?.textContent.trim();
                const address = cells[2]?.textContent.trim();

                // Nur Datapoints mit Name und Address in das Array aufnehmen
                if (name && address) {
                    return {
                        id: id || null, // ID kann optional sein
                        name,
                        address,
                    };
                }
                return null; // Leere Zeile wird ignoriert
            })
            .filter((datapoint) => datapoint !== null); // Null-Werte entfernen


        // Daten an den Server senden
        const payload = {
            id : deviceId,
            name: deviceName,
            type: deviceType,
            config,
            datapoints,
        };

        console.log('Payload:', payload);

        const response = await fetch(`/api/device/${deviceId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(payload),
        });

        if (!response.ok) {
            throw new Error(`Failed to update device. Status: ${response.status}`);
        }

        const result = await response.json();
        console.log('Device updated successfully:', result);

        // Modal schließen und UI aktualisieren (optional)
        const modal = bootstrap.Modal.getInstance(document.getElementById('modal-edit-device'));
        modal.hide();
        alert('Device updated successfully!');
    } catch (error) {
        console.error('Error saving device:', error);
        alert('Failed to update the device. Please try again.');
    }
}

document.getElementById('btn-add-new-device').addEventListener('click', async function () {
    await saveDevice();
});
