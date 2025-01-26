// const BASE_PATH = 'http://localhost:7777';
// const WS_PATH = 'ws://localhost:7777';

initWebSocket(WS_PATH);

// Funktion zum Aktualisieren der Device-Tabellen
function updateDeviceTables(devices) {
    devices.forEach(device => {
        const tableBody = document.querySelector(`#device-${device.device_id}-table`);

        if (tableBody) {
            tableBody.innerHTML = ''; // Vorhandene Datenpunkte entfernen

            device.datapoints.forEach(datapoint => {
                const row = document.createElement('tr');

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
                
                // Live-Daten in den Chart aktualisieren
                if (liveCharts[datapoint.id]) {
                    updateChart(liveCharts[datapoint.id], datapoint.value);
                }
            });
        }
    });
}

// Funktion zum Abrufen und Befüllen der Devices beim Laden der Seite
async function fetchAndPopulateDevices() {
    try {
        // API-Aufruf
        // const response = await fetch(`${BASE_PATH}/getDevices`);
        const response = await fetch(`/api/getDevices`);

        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }

        // JSON-Daten abrufen
        const devicesArr = await response.json();

        const devices = devicesArr.devices; 

        // console.log('Devices fetched successfully:', devices);

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
            statusIcon.style.width = '10px';
            statusIcon.style.height = '10px';
            statusIcon.style.borderRadius = '50%';
            statusIcon.style.marginRight = '5px';

            if (device.status === 'running') {
                statusIcon.style.backgroundColor = 'green';
                statusIcon.title = 'running';
            } else if (device.status === 'stopped') {
                statusIcon.style.backgroundColor = 'orange';
                statusIcon.title = 'stopped';
            } else if (device.status === 'error') {
                statusIcon.style.backgroundColor = 'red';
                statusIcon.title = 'error';
            }

            statusCell.appendChild(statusIcon);
            row.appendChild(statusCell);

            // Actions
            const actionsCell = document.createElement('td');
            actionsCell.className = 'text-center align-middle';
            actionsCell.style.height = '60px';

            const editButton = document.createElement('a');
            editButton.className = 'btn btnMaterial btn-flat success semicircle';
            editButton.href = '#';
            editButton.innerHTML = '<i class="fas fa-pen"></i>';
            editButton.setAttribute('data-bs-toggle', 'modal');
            editButton.setAttribute('data-bs-target', '#modal-edit-device');

            // Event-Listener für das Öffnen des Bearbeiten-Modals
            editButton.addEventListener('click', () => {
                initializeEditDeviceModal(device.id); // Übergibt die aktuelle `device_id`
            }); 

            actionsCell.appendChild(editButton);

            const deleteButton = document.createElement('a');
            deleteButton.className = 'btn btnMaterial btn-flat accent btnNoBorders checkboxHover';
            deleteButton.style.marginLeft = '5px';
            deleteButton.setAttribute('data-bs-toggle', 'modal');
            deleteButton.setAttribute('data-bs-target', '#delete-modal');
            deleteButton.innerHTML = '<i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>';
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

// Füge den Event Listener hinzu, der beim Öffnen des Edit-Modals aktiviert wird
document.getElementById('modal-edit-device').addEventListener('show.bs.modal', initializeEditDeviceModal);