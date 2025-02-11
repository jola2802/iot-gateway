let wsDevices;

// Funktion zum Initialisieren der WebSocket-Verbindung
function initWebSocket(WS_PATH) {
    // wsDevices = new WebSocket(`${WS_PATH}/deviceData`);
    wsDevices = new WebSocket(`/api/ws-device-data`);

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

document.getElementById('btn-add-new-device').addEventListener('click', async () => {
    try {
        // Erfassung der Eingabewerte
        const deviceData = {
            deviceName: document.getElementById('device-name').value,
            deviceType: document.getElementById('select-device-type').value,
            address: document.getElementById('address')?.value || document.querySelector('#s7-config [placeholder="192.168.2.100:102"]')?.value ||'',
            securityPolicy: document.getElementById('select-security-policy')?.value || '',
            securityMode: document.getElementById('select-security-mode')?.value || '',
            acquisitionTime: parseInt(document.getElementById('acquisition-time-opc-ua')?.value || 
                                      document.getElementById('acquisition-time-s7')?.value || '0', 10),
            username: document.getElementById('username')?.value || '',
            password: document.getElementById('password')?.value || '',
            rack: document.querySelector('#s7-config [placeholder="0"]')?.value || '',
            slot: document.querySelector('#s7-config [placeholder="1"]')?.value || '',
        };

        console.log('Neues Gerät:', deviceData);

        // API-Request senden
        const response = await fetch('/api/add-device', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(deviceData),
        });

        if (!response.ok) {
            throw new Error(`Fehler beim Hinzufügen des Geräts: ${response.statusText}`);
        }

        const result = await response.json();
        alert('Gerät erfolgreich hinzugefügt!');

        // Modal schließen und Seite aktualisieren
        // document.getElementById('modal-new-device').modal('hide');
        location.reload(); // Alternativ: Nur die Tabelle aktualisieren
        
    } catch (error) {
        console.error('Fehler beim Speichern des Geräts:', error);
        alert('Fehler beim Hinzufügen des Geräts. Bitte versuchen Sie es erneut.');
    }
});


document.getElementById('btn-edit-device').onclick=async () => {
    await saveEditDevice();
};

async function saveEditDevice() {
    try {
        // Erfassung der Eingabewerte
        const deviceData = {
            // deviceId: document.getElementById('device-name-1').dataset.deviceId, // Nehme die ID aus einem Attribut, falls vorhanden
            deviceName: document.getElementById('device-name-1').value,
            deviceType: document.getElementById('select-device-type-1').value,
            address: document.getElementById('address-1')?.value || document.getElementById('address-2')?.value || '',
            securityPolicy: document.getElementById('select-security-policy-1')?.value || '',
            securityMode: document.getElementById('select-security-mode-1')?.value || '',
            acquisitionTime: parseInt(
                document.getElementById('acquisition-time-opc-ua-1')?.value ||
                document.getElementById('acquisition-time-2')?.value || '0',
                10
            ),
            username: document.getElementById('username')?.value || '',
            password: document.getElementById('password')?.value || '',
            rack: document.querySelector('#s7-config-1 [placeholder="0"]')?.value || '',
            slot: document.querySelector('#s7-config-1 [placeholder="1"]')?.value || '',
            datapoints: Array.from(document.querySelectorAll('#ipi-table tbody tr')).map(row => {
                // Alle Zellen in der Zeile abrufen
                const cells = row.querySelectorAll('td');
            
                // Prüfen, ob die Zeile Eingabefelder hat (für neue Datenpunkte)
                const nameInput = cells[1]?.querySelector('input');
                const datatypeInput = cells[2]?.querySelector('select');
                const addressInput = cells[3]?.querySelector('input');

                const isOpcUa = document.getElementById('select-device-type-1').value === 'opc-ua';
                return {
                    datapointId: row.querySelector('td').textContent.trim(),
                    name: nameInput ? nameInput.value.trim() : cells[1]?.textContent.trim() || '',
                    datatype: isOpcUa ? '' : cells[3]?.textContent.trim() || '',
                    address: isOpcUa ? cells[3]?.textContent.trim() || '' : cells[3]?.textContent.trim() || '',
                };
            }),
        };

        deviceData.deviceId = localStorage.getItem('device_id');

        console.log('Zu aktualisierende Gerätedaten:', deviceData);

        // API-Request senden
        const response = await fetch(`/api/update-device/${deviceData.deviceId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(deviceData),
        });

        if (!response.ok) {
            throw new Error(`Fehler beim Aktualisieren des Geräts: ${response.statusText}`);
        }

        const result = await response.json();
        alert('Gerät erfolgreich aktualisiert!');

        // Modal schließen und Seite aktualisieren
        const modalEl = document.getElementById('modal-edit-device');
        // Versuche, eine bestehende Instanz abzurufen
        let modalInstance = bootstrap.Modal.getInstance(modalEl);
        if (!modalInstance) {
            // Falls keine Instanz existiert, erstelle eine neue
            modalInstance = new bootstrap.Modal(modalEl);
        }
        modalInstance.hide();


        // document.getElementById('modal-edit-device').modal('hide');
        location.reload(); // Alternativ: Nur die Tabelle aktualisieren
    } catch (error) {
        console.error('Fehler beim Aktualisieren des Geräts:', error);
        alert('Fehler beim Aktualisieren des Geräts. Bitte versuchen Sie es erneut.');
    }
}