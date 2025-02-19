initWebSocket();

// Funktion zum Aktualisieren der Device-Tabellen
function updateDeviceTables(devices) {
    devices.forEach(device => {
        const tableBody = document.querySelector(`#device-${device.device_id}-table`);

        if (tableBody) {
            // tableBody.innerHTML = ''; // Vorhandene Datenpunkte entfernen

            // Wenn Datenpunkt bereits vorhanden, dann nur last value aktualisieren
            const existingDatapoints = tableBody.querySelectorAll('tr');
            const existingDatapointIds = new Set(Array.from(existingDatapoints).map(row => {
                const idCell = row.querySelector('td:first-child');
                return idCell ? idCell.textContent : null; // Handle case where idCell is null
            }));

            // Neue Datapoints hinzufügen oder bestehende aktualisieren
            device.datapoints.forEach(datapoint => {
                const existingRow = tableBody.querySelector(`tr[data-datapoint-id="${datapoint.id}"]`);

                if (existingDatapointIds.has(datapoint.id)) {
                    // Datapoint existiert bereits, nur Wert aktualisieren
                    const valueCell = existingRow.querySelector('td:nth-child(3)');
                    valueCell.textContent = datapoint.value;

                    // Chart Button hinzufügen (auch für bestehende Datapoints)
                    const chartCell = existingRow.querySelector('td:last-child'); // Selektiere die letzte Zelle (Chart-Zelle)
                    if (!chartCell) { //Nur hinzufügen wenn noch nicht vorhanden
                        chartCell = document.createElement('td');
                        const chartButton = document.createElement('button');
                        chartButton.className = 'btn btn-outline-secondary';
                        chartButton.innerHTML = '<i class="typcn typcn-chart-line"></i>';
                        chartButton.addEventListener('click', () => openChartModal(device.device_id, datapoint));
                        chartCell.appendChild(chartButton);
                        existingRow.appendChild(chartCell);
                    }

                    updateChart(datapoint, datapoint.value, new Date());
                } else {
                    // Datapoint existiert noch nicht, neue Zeile erstellen
                    const row = document.createElement('tr');
                    row.setAttribute('data-datapoint-id', datapoint.id); // Datapoint ID als Attribut hinzufügen

                    // Datapoint ID
                    const idCell = document.createElement('td');
                    idCell.textContent = datapoint.id;
                    row.appendChild(idCell);

                    // Datapoint Name
                    const nameCell = document.createElement('td');
                    nameCell.textContent = datapoint.name;
                    row.appendChild(nameCell);

                    // Last Value
                    const valueCell = document.createElement('td');
                    valueCell.textContent = datapoint.value;
                    row.appendChild(valueCell);

                    // Chart Button
                    const chartCell = document.createElement('td');
                    const chartButton = document.createElement('button');
                    chartButton.className = 'btn btn-outline-secondary';
                    chartButton.innerHTML = '<i class="typcn typcn-chart-line"></i>';
                    chartButton.addEventListener('click', () => openChartModal(device.device_id, datapoint));
                    chartCell.appendChild(chartButton);
                    row.appendChild(chartCell);

                    tableBody.appendChild(row);

                    updateChart(datapoint, datapoint.value, new Date());
                }
            });
        }


    });
}

// Funktion zum Abrufen und Befüllen der Devices beim Laden der Seite
async function fetchAndPopulateDevices() {
    try {
        // warte 200ms 
        await new Promise(resolve => setTimeout(resolve, 200));
        // API-Aufruf
        const response = await fetch(`/api/getDevices`);

        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }

        // JSON-Daten abrufen
        const devicesArr = await response.json();

        const devices = devicesArr.devices; 

        // Tabelle referenzieren
        const tableBody = document.querySelector('#table-devices tbody');

        // Spinner-Zeile entfernen
        tableBody.innerHTML = '';
        

        // Accordion-Container referenzieren
        const accordionContainer = document.querySelector('#accordion-data');

        // Geräte-Daten in Tabelle und Accordion einfügen
        devices.forEach(device => {
            // **Obere Tabelle**
            const row = document.createElement('tr');

            // Device ID
            const idCell = document.createElement('td');
            idCell.textContent = device.id;
            row.appendChild(idCell);

            // Device Name
            const deviceCell = document.createElement('td');
            deviceCell.textContent = device.deviceName;
            row.appendChild(deviceCell);
            
            // **Type** (Neue Spalte hinzufügen)
            const typeCell = document.createElement('td');
            typeCell.textContent = device.deviceType; // Stellen Sie sicher, dass 'type' im Gerätedatenobjekt vorhanden ist
            row.appendChild(typeCell);

            // Address
            const addressCell = document.createElement('td');
            addressCell.textContent = device.address;
            row.appendChild(addressCell);

            // Status
            const statusCell = document.createElement('td');
            const statusIcon = document.createElement('span');
            statusIcon.className = 'status-lamp';
            statusIcon.style.display = 'inline-block';
            statusIcon.style.width = '15px';
            statusIcon.style.height = '15px';
            statusIcon.style.borderRadius = '50%';
            statusIcon.style.marginRight = '5px';

            if (device.status === '1 (running)') {
                statusIcon.style.backgroundColor = 'green';
                statusIcon.title = 'running';
            } else if (device.status === 'stopped') {
                statusIcon.style.backgroundColor = 'orange';
                statusIcon.title = 'stopped';
            } else if (device.status === 'error') {
                statusIcon.style.backgroundColor = 'red';
                statusIcon.title = 'error';
            }else if (device.status === '3 (initializing)') {
                statusIcon.style.backgroundColor = 'white';
                statusIcon.style.border = '3px solid gray';
                statusIcon.title = 'initializing';
            } else if (device.status === '4 (no datapoints)') {
                // Rahmen grün, Füllung leer
                statusIcon.style.backgroundColor = 'white';
                statusIcon.style.border = '3px solid green';
                statusIcon.title = 'no datapoints';
            } else if (device.status === '5 (no connection)') {
                // Rahmen rot, Füllung leer
                statusIcon.style.backgroundColor = 'white';
                statusIcon.style.border = '3px solid red';
                statusIcon.title = 'no connection';
            } else {
                statusIcon.style.backgroundColor = 'red';
                statusIcon.title = 'error';
            }

            statusCell.appendChild(statusIcon);
            row.appendChild(statusCell);

            // Actions
            const actionsCell = document.createElement('td');
            actionsCell.className = 'text-center align-middle';
            actionsCell.style.height = '60px';

            // Restart Button hinzufügen
            const restartButton = document.createElement('a');
            restartButton.className = 'btn btnMaterial btn-flat primary semicircle';
            restartButton.innerHTML = '<i class="fas fa-sync-alt"></i>';
            restartButton.style.marginRight = '5px';
            //Tooltip hinzufügen
            restartButton.setAttribute('data-bs-toggle', 'tooltip');
            restartButton.setAttribute('data-bs-placement', 'top');
            restartButton.setAttribute('title', 'Restart Device');
            
            // Event-Listener für den Restart-Button
            restartButton.addEventListener('click', async () => {
                try {
                    const response = await fetch(`/api/restart-device/${device.id}`, {
                        method: 'POST'
                    });
                    if (response.ok) {
                        alert('Driver has been restarted');
                        // Device-Tabelle aktualisieren
                        fetchAndPopulateDevices();
                    } else {
                        throw new Error('Error restarting driver');
                    }
                } catch (error) {
                    console.error('Error:', error);
                    alert('Error restarting driver');
                }
            });

            actionsCell.appendChild(restartButton);

            const editButton = document.createElement('a');
            editButton.className = 'btn btnMaterial btn-flat success semicircle';
            editButton.innerHTML = '<i class="fas fa-pen"></i>';
            editButton.setAttribute('data-bs-toggle', 'modal');
            editButton.setAttribute('data-bs-target', '#modal-edit-device');

            // Event-Listener für das Öffnen des Bearbeiten-Modals
            editButton.addEventListener('click', () => {
                // Speichere die device_id als data-Attribut am Modal
                document.getElementById('modal-edit-device').setAttribute('data-device-id', device.id);
                // console.log('Device ID bei edit button:', device.id);
            });

            actionsCell.appendChild(editButton);

            const deleteButton = document.createElement('a');
            deleteButton.className = 'btn btnMaterial btn-flat accent btnNoBorders checkboxHover';
            deleteButton.style.marginLeft = '5px';
            // deleteButton.setAttribute('data-bs-toggle', 'modal');
            // deleteButton.setAttribute('data-bs-target', '#delete-modal');
            deleteButton.innerHTML = '<i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>';

            // Klickevent-Listener, der die oben definierte Löschfunktion aufruft
            deleteButton.addEventListener('click', (event) => {
                event.preventDefault();
                confirmDeleteDevice(device.id); // Hier wird das jeweilige Gerät anhand seiner ID gelöscht
            });

            actionsCell.appendChild(deleteButton);

            row.appendChild(actionsCell);
            tableBody.appendChild(row);

            // **Accordion-Struktur**
            if (!document.querySelector(`#device-${device.id}`)) {
                const accordionItem = document.createElement('div');
                accordionItem.className = 'accordion-item';
                accordionItem.id = `device-${device.id}`;

                accordionItem.innerHTML = `
                    <h2 class="accordion-header" role="tab">
                        <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#device-${device.id}-body" aria-expanded="false" aria-controls="device-${device.id}-body">
                            ${device.deviceName}
                        </button>
                    </h2>
                    <div id="device-${device.id}-body" class="accordion-collapse collapse" role="tabpanel" data-bs-parent="#accordion-data">
                        <div class="accordion-body">
                            <div class="table-responsive">
                                <table class="table">
                                    <thead>
                                        <tr>
                                            <th>Datapoint ID</th>
                                            <th>Datapoint</th>
                                            <th>Last Value</th>
                                            <th>Chart</th>
                                        </tr>
                                    </thead>
                                    <tbody id="device-${device.id}-table">
                                        <!-- Dynamische Datenpunkte -->
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    </div>
                `;
                accordionContainer.appendChild(accordionItem);
            }
        });
    } catch (error) {
        console.error('Error fetching devices:', error);
    }
}

const opcUaConfig = document.getElementById('opc-ua-config');
const s7Config = document.getElementById('s7-config');
const mqttConfig = document.getElementById('mqtt-config');

// Funktion aufrufen, um die Tabelle und Accordion-Struktur zu befüllen
fetchAndPopulateDevices();
hideAllConfigs();

// #########
// Modal new device dynamisieren
// #########

// Füge den Event Listener hinzu, der beim Öffnen des Modals aktiviert wird
document.getElementById('modal-new-device').addEventListener('show.bs.modal', initializeNewDeviceModal);

// #########
// Modal edit device dynamisieren
// #########

// Event-Listener für Edit-Buttons
function setupEditButtonListener() {
    document.removeEventListener('click', handleEditButtonClick); // Alten Listener entfernen
    document.addEventListener('click', handleEditButtonClick); // Neuen Listener hinzufügen
}

async function handleEditButtonClick(event) {
    const editButton = event.target.closest('.btn-flat.success');
    if (editButton) {
        event.preventDefault();
        
        // Device ID aus dem Button-Attribut holen
        const deviceId = document.getElementById('modal-edit-device').getAttribute('data-device-id');
        
        if (deviceId) {
            try {
                // Warte bis die Daten geladen sind
                await initializeEditDeviceModal(deviceId);
                
                // Erst dann das Modal öffnen
                const modal = new bootstrap.Modal(document.getElementById('modal-edit-device'));
                modal.show();
            } catch (error) {
                console.error('Fehler beim Laden der Gerätedaten:', error);
                alert('Fehler beim Laden der Gerätedaten. Bitte versuchen Sie es erneut.');
            }
        }
    }
}

// // Initialisiere den Event-Listener einmal
setupEditButtonListener();

