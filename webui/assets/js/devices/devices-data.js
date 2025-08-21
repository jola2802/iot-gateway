let wsDevices;

// Funktion zum Initialisieren der WebSocket-Verbindung
async function initWebSocket() {
    const response = await fetch ("/api/ws-token", {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
        },
    });

    const token = await response.json();

    wsDevices = new WebSocket(`/api/ws-device-data?token=${token.token}`);

    wsDevices.onopen = () => {
        console.log('WebSocket-Verbindung hergestellt');
    };

    wsDevices.onmessage = (event) => {
        const devices = JSON.parse(event.data);
        updateDeviceTables(devices);
        // updateChart(devices);
    };

    wsDevices.onerror = (error) => {
        console.error('WebSocket-Fehler:', error);
        window.location.reload();
    };

    wsDevices.onclose = (event) => {
        console.warn('WebSocket-Verbindung geschlossen:', event.reason);
        window.location.reload();
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

        // Prüfen, ob alle Felder ausgefüllt sind
        if (deviceData.deviceType === 'opc-ua') {
            if (!deviceData.deviceName || !deviceData.address || !deviceData.securityPolicy || !deviceData.securityMode || !deviceData.acquisitionTime) {
                alert('Bitte füllen Sie alle Felder aus.');
                return;
            }
        } else if (deviceData.deviceType === 's7') {
            if (!deviceData.deviceName || !deviceData.address || !deviceData.rack || !deviceData.slot) {
                alert('Bitte füllen Sie alle Felder aus.');
                return;
            }
        }

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

        // alert('Gerät erfolgreich hinzugefügt!');

        // Modal schließen und Seite aktualisieren
        // document.getElementById('modal-new-device').modal('hide');
        window.location.reload(); // Alternativ: Nur die Tabelle aktualisieren
        
    } catch (error) {
        console.error('Fehler beim Speichern des Geräts:', error);
        alert('Fehler beim Hinzufügen des Geräts. Bitte versuchen Sie es erneut.');
    }
});

document.getElementById('btn-edit-device').onclick = async () => {
    saveEditDevice(); // 5 Sekunden warten
    // window.location.reload();
};

function saveEditDevice() {
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
            username: document.getElementById('username')?.value || document.getElementById('username-1')?.value || '',
            password: document.getElementById('password')?.value || document.getElementById('password-1')?.value || '',
            rack: document.querySelector('#rack')?.value || document.querySelector('#s7-config-1 [placeholder="0"]')?.value || '',
            slot: document.querySelector('#slot')?.value || document.querySelector('#s7-config-1 [placeholder="1"]')?.value || '',
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
                    datatype: isOpcUa ? '' : datatypeInput ? datatypeInput.value.trim() : cells[2]?.textContent.trim() || '',
                    address: addressInput ? addressInput.value.trim() : cells[3]?.textContent.trim() || '',
                };
            }),
        };

        deviceData.deviceId = localStorage.getItem('device_id');

        console.log('Zu aktualisierende Gerätedaten:', deviceData);

        // API-Request senden
        fetch(`/api/update-device/${deviceData.deviceId}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(deviceData),
        }).then(response => {
            if (!response.ok) {
                throw new Error(`Fehler beim Aktualisieren des Geräts: ${response.statusText}`);
            }
        }).catch(error => {
            console.error('Fehler beim Aktualisieren des Geräts:', error);
        });

        // Modal schließen
        const modalEl = document.getElementById('modal-edit-device');
        const modalInstance = bootstrap.Modal.getInstance(modalEl);
        if (modalInstance) {
            modalInstance.hide();
        }
    } catch (error) {
        console.error('Fehler beim Aktualisieren des Geräts:', error);
        // alert('Fehler beim Aktualisieren des Geräts. Bitte versuchen Sie es erneut.');
    }
}