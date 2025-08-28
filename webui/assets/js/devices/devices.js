const opcUaConfig = document.getElementById('opc-ua-config');
const s7Config = document.getElementById('s7-config');
const mqttConfig = document.getElementById('mqtt-config');

initWebSocket();

// Konstante für Statusfarben
const STATUS_COLORS = {
    running: 'green',
    stopped: 'orange',
    error: 'red',
    initializing: 'white',
    noDatapoints: 'white',
    noConnection: 'white',
    connectionLost: 'orange'
};

// Funktion zum Erstellen eines Status-Icons
function createStatusIcon(status) {
    const statusIcon = document.createElement('span');
    statusIcon.className = 'status-lamp';
    statusIcon.style.display = 'inline-block';
    statusIcon.style.width = '15px';
    statusIcon.style.height = '15px';
    statusIcon.style.borderRadius = '50%';
    statusIcon.style.marginRight = '5px';

    // Konvertiere numerische Status in String-Format
    if (typeof status === 'number') {
        switch(status) {
            case 1: status = '1 (running)'; break;
            case 0: status = '0 (stopped)'; break;
            case 2: status = '2 (initializing)'; break;
            case 3: status = '3 (error)'; break;
            case 4: status = '4 (no datapoints)'; break;
            case 5: status = '5 (no connection)'; break;
            case 6: status = '6 (connection lost)'; break;
        }
    }

    switch (status) {
        case '1 (running)':
            statusIcon.style.backgroundColor = STATUS_COLORS.running;
            statusIcon.title = 'running';
            break;
        case '0 (stopped)':
            statusIcon.style.backgroundColor = STATUS_COLORS.stopped;
            statusIcon.title = 'stopped';
            break;
        case '3 (error)':
            statusIcon.style.backgroundColor = STATUS_COLORS.error;
            statusIcon.title = 'error';
            break;
        case '2 (initializing)':
            statusIcon.style.backgroundColor = STATUS_COLORS.initializing;
            statusIcon.style.border = '2px solid gray';
            statusIcon.title = 'initializing';
            break;
        case '4 (no datapoints)':
            statusIcon.style.backgroundColor = STATUS_COLORS.noDatapoints;
            statusIcon.style.border = '5px solid grey';
            statusIcon.title = 'no datapoints';
            break;
        case '5 (no connection)':
            statusIcon.style.backgroundColor = STATUS_COLORS.noConnection;
            statusIcon.style.border = '4px solid red';
            statusIcon.title = 'no connection';
            break;
        case '6 (connection lost)':
            statusIcon.style.backgroundColor = STATUS_COLORS.connectionLost;
            statusIcon.title = 'connection lost';
            break;
        default:
            console.warn('Unknown status:', status);
            statusIcon.style.backgroundColor = STATUS_COLORS.error;
            statusIcon.title = 'error';
    }

    return statusIcon;
}

// Funktion zum Erstellen eines Gerätereihen
function createDeviceRow(device) {
    const row = document.createElement('tr');
    row.setAttribute('data-device-id', device.id);

    // Device ID
    const idCell = document.createElement('td');
    idCell.textContent = device.id;
    idCell.style.textAlign = 'center';
    idCell.style.verticalAlign = 'middle';
    row.appendChild(idCell);

    // Device Name
    const deviceCell = document.createElement('td');
    deviceCell.textContent = device.deviceName;
    deviceCell.style.fontWeight = 'bold';
    deviceCell.style.width = 'break-all';
    deviceCell.style.verticalAlign = 'middle';
    row.appendChild(deviceCell);

    // Type
    const typeCell = document.createElement('td');
    typeCell.textContent = device.deviceType;
    typeCell.style.textAlign = 'center';
    typeCell.style.verticalAlign = 'middle';
    row.appendChild(typeCell);

    // Address
    const addressCell = document.createElement('td');
    addressCell.textContent = device.address;
    addressCell.style.fontWeight = 'bold';
    addressCell.style.width = 'break-all';
    addressCell.style.textAlign = 'center';
    addressCell.style.verticalAlign = 'middle';
    row.appendChild(addressCell);

    // Acquisition Time
    const acquisitionTimeCell = document.createElement('td');
    acquisitionTimeCell.textContent = device.acquisitionTime;
    acquisitionTimeCell.style.textAlign = 'center';
    acquisitionTimeCell.style.verticalAlign = 'middle';
    row.appendChild(acquisitionTimeCell);

    // Status
    const statusCell = document.createElement('td');
    const statusIcon = createStatusIcon(device.status);
    statusCell.appendChild(statusIcon);
    statusCell.style.textAlign = 'center';
    statusCell.style.verticalAlign = 'middle';
    row.appendChild(statusCell);

    // Actions
    const actionsCell = createActionsCell(device);
    row.appendChild(actionsCell);

    return row;
}

// Funktion zum Erstellen der Aktionszelle
function createActionsCell(device) {
    const actionsCell = document.createElement('td');
    actionsCell.className = 'text-center align-middle';
    actionsCell.style.height = '50px';

    // Restart Button
    const restartButton = createRestartButton(device);
    actionsCell.appendChild(restartButton);

    // Edit Button
    const editButton = createEditButton(device);
    actionsCell.appendChild(editButton);

    // Delete Button
    const deleteButton = createDeleteButton(device);
    actionsCell.appendChild(deleteButton);

    return actionsCell;
}

// Funktion zum Erstellen des Restart-Buttons
function createRestartButton(device) {
    const restartButton = document.createElement('a');
    restartButton.className = 'btn btnMaterial btn-flat primary semicircle';
    restartButton.innerHTML = '<i class="fas fa-sync-alt"></i>';
    restartButton.style.marginRight = '5px';
    restartButton.setAttribute('data-bs-toggle', 'tooltip');
    restartButton.setAttribute('data-bs-placement', 'top');
    restartButton.setAttribute('title', 'Restart Device');

    restartButton.addEventListener('click', async () => {
        try {
            const response = await fetch(`/api/restart-device/${device.id}`, {
                method: 'POST'
            });
            if (response.ok) {
                alert('Driver has been restarted');
                fetchAndPopulateDevices();
            } else {
                throw new Error('Error restarting driver');
            }
        } catch (error) {
            console.error('Error:', error);
            alert('Error restarting driver');
        }
    });

    return restartButton;
}

// Funktion zum Erstellen des Edit-Buttons (nur wenn nicht type = mqtt)
function createEditButton(device) {
    const editButton = document.createElement('a');
    editButton.className = 'btn btnMaterial btn-flat success semicircle';
    editButton.innerHTML = '<i class="fas fa-pen"></i>';
    editButton.setAttribute('data-bs-toggle', 'modal');
    editButton.setAttribute('data-bs-target', '#modal-edit-device');
    editButton.setAttribute('title', 'Edit Device Configuration');

    editButton.addEventListener('click', () => {
        document.getElementById('modal-edit-device').setAttribute('data-device-id', device.id);
    });

    return editButton;
}


// Funktion zum Erstellen des Delete-Buttons
function createDeleteButton(device) {
    const deleteButton = document.createElement('a');
    deleteButton.className = 'btn btnMaterial btn-flat accent btnNoBorders checkboxHover';
    deleteButton.style.marginLeft = '5px';
    deleteButton.innerHTML = '<i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>';
    deleteButton.setAttribute('title', 'Delete Device');

    deleteButton.addEventListener('click', (event) => {
        event.preventDefault();
        confirmDeleteDevice(device.id);
    });
    return deleteButton;
}

// Funktion zum Abrufen und Befüllen der Devices beim Laden der Seite
async function fetchAndPopulateDevices() {
    try {
        await new Promise(resolve => setTimeout(resolve, 300));
        const response = await fetch(`/api/getDevices`);

        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }

        const devicesArr = await response.json();
        const devices = devicesArr.devices; 

        const tableBody = document.querySelector('#table-devices tbody');
        tableBody.innerHTML = '';

        const accordionContainer = document.querySelector('#accordion-data');

        devices.forEach(device => {
            const row = createDeviceRow(device);
            tableBody.appendChild(row);

            if (!document.querySelector(`#device-${device.id}`)) {
                const accordionItem = createAccordionItem(device);
                accordionContainer.appendChild(accordionItem);
            }
        });
    } catch (error) {
        console.error('Error fetching devices:', error);
    }
}

// Funktion zum Erstellen eines Accordion-Items
function createAccordionItem(device) {
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
                    <table class="table" style="width: 100%;">
                        <thead>
                            <tr>
                                <th>Datapoint ID</th>
                                <!-- <th>Datapoint</th> -->
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
    return accordionItem;
}

// Funktion zum Aktualisieren der Device-Tabellen
function updateDeviceTables(devices) {
    devices.forEach(device => {
        // Konvertiere device_id zu einer normalen ID wenn nötig
        const deviceId = device.device_id || device.id;
        
        const tableBody = document.querySelector(`#device-${deviceId}-table`);

        if (tableBody) {
            // Wenn Datenpunkt bereits vorhanden, dann nur last value aktualisieren
            const existingDatapoints = tableBody.querySelectorAll('tr');
            const existingDatapointIds = new Set(Array.from(existingDatapoints).map(row => {
                const idCell = row.querySelector('td:first-child');
                return idCell ? idCell.textContent : null;
            }));

            // Neue Datapoints hinzufügen oder bestehende aktualisieren
            device.datapoints.forEach(datapoint => {
                const existingRow = tableBody.querySelector(`tr[data-datapoint-id="${datapoint.id}"]`);

                if (existingDatapointIds.has(datapoint.id)) {
                    // Datapoint existiert bereits, nur Wert aktualisieren
                    const valueCell = existingRow.querySelector('td:nth-child(2)');
                    valueCell.textContent = datapoint.value;

                    // Chart Button hinzufügen (auch für bestehende Datapoints)
                    const chartCell = existingRow.querySelector('td:last-child');
                    if (!chartCell) {
                        const newChartCell = document.createElement('td');
                        const chartButton = document.createElement('button');
                        chartButton.className = 'btn btn-outline-secondary';
                        chartButton.innerHTML = '<i class="typcn typcn-chart-line"></i>';
                        chartButton.addEventListener('click', () => openChartModal(deviceId, datapoint));
                        newChartCell.appendChild(chartButton);
                        existingRow.appendChild(newChartCell);
                    }

                    updateChart(datapoint, datapoint.value, new Date());
                } else {
                    // Datapoint existiert noch nicht, neue Zeile erstellen
                    const row = document.createElement('tr');
                    row.setAttribute('data-datapoint-id', datapoint.id);

                    // Datapoint ID
                    const idCell = document.createElement('td');
                    idCell.textContent = datapoint.id;
                    row.appendChild(idCell);

                    // Last Value
                    const valueCell = document.createElement('td');
                    valueCell.textContent = datapoint.value;
                    row.appendChild(valueCell);

                    // Chart Button
                    const chartCell = document.createElement('td');
                    const chartButton = document.createElement('button');
                    chartButton.className = 'btn btn-outline-secondary';
                    chartButton.innerHTML = '<i class="typcn typcn-chart-line"></i>';
                    chartButton.addEventListener('click', () => openChartModal(deviceId, datapoint));
                    chartCell.appendChild(chartButton);
                    row.appendChild(chartCell);

                    tableBody.appendChild(row);

                    updateChart(datapoint, datapoint.value, new Date());
                }
            });
        }

        // Update der Device-Tabelle mit Status
        const deviceRow = document.querySelector(`#table-devices tr[data-device-id="${deviceId}"]`);
        if (deviceRow) {
            const statusCell = deviceRow.querySelector('td:nth-child(6)'); // Die Status-Spalte ist die 6.
            if (statusCell) {
                statusCell.innerHTML = ''; // Lösche den alten Status
                const statusIcon = createStatusIcon(device.status);
                statusCell.appendChild(statusIcon);
            }
        }
    });
}

// Funktion aufrufen, um die Tabelle und Accordion-Struktur zu befüllen
fetchAndPopulateDevices();
hideAllConfigs();

// Event-Listener für Modal-Events hinzufügen
document.getElementById('modal-edit-device').addEventListener('hidden.bs.modal', function () {
    // Modal vollständig zurücksetzen wenn es geschlossen wird
    resetEditModal();
});

document.getElementById('modal-edit-device').addEventListener('hide.bs.modal', function () {
    // Seite aktualisieren wenn Modal geschlossen wird
    setTimeout(() => {
        fetchAndPopulateDevices();
    }, 100);
});

// #########
// Modal new device dynamisieren
// #########

// Füge den Event Listener hinzu, der beim Öffnen des Modals aktiviert wird
document.getElementById('modal-new-device').addEventListener('show.bs.modal', initializeNewDeviceModal);

// #########
// Modal edit device dynamisieren
// #########

// Einmaliger Event-Listener für Edit-Buttons mit korrekter Modal-Behandlung
document.addEventListener('click', async function(event) {
    const editButton = event.target.closest('.btn-flat.success');
    if (editButton && editButton.getAttribute('data-bs-target') === '#modal-edit-device') {
        event.preventDefault();
        event.stopPropagation();
        
        // Device ID aus dem Button-Attribut holen
        const modalElement = document.getElementById('modal-edit-device');
        const deviceId = modalElement.getAttribute('data-device-id');
        
        if (deviceId) {
            try {
                // Modal-Element vollständig zurücksetzen
                resetEditModal();
                
                // Warte bis die Daten geladen sind
                await initializeEditDeviceModal(deviceId);
                
                // Prüfe ob bereits eine Modal-Instanz existiert
                let modal = bootstrap.Modal.getInstance(modalElement);
                if (!modal) {
                    modal = new bootstrap.Modal(modalElement, {
                        backdrop: 'static',
                        keyboard: false
                    });
                }
                
                // Erst dann das Modal öffnen
                modal.show();
            } catch (error) {
                console.error('Fehler beim Laden der Gerätedaten:', error);
                alert('Fehler beim Laden der Gerätedaten. Bitte versuchen Sie es erneut.');
            }
        }
    }
});

// Funktion zum vollständigen Zurücksetzen des Edit-Modals
function resetEditModal() {
    const modalElement = document.getElementById('modal-edit-device');
    
    // Entferne alle bestehenden Modal-Instanzen
    const existingModal = bootstrap.Modal.getInstance(modalElement);
    if (existingModal) {
        existingModal.dispose();
    }
    
    // Lösche localStorage
    localStorage.removeItem('device_id');
    
    // Zurücksetzen aller Formulare
    const form = modalElement.querySelector('#edit-device-form');
    if (form) {
        form.reset();
    }
    
    // Leere die Datenpunkt-Tabelle
    const datapointsTableBody = document.querySelector('#ipi-table tbody');
    if (datapointsTableBody) {
        datapointsTableBody.innerHTML = '';
    }
    
    // Entferne alle Event-Listener vom Browse-Button
    const browseButton = document.getElementById('browse-nodes-btn');
    if (browseButton) {
        const newButton = browseButton.cloneNode(true);
        browseButton.parentNode.replaceChild(newButton, browseButton);
    }
    
    // Entferne Event-Listener vom Save-Button
    const saveButton = document.getElementById('btn-edit-device');
    if (saveButton) {
        const newSaveButton = saveButton.cloneNode(true);
        saveButton.parentNode.replaceChild(newSaveButton, saveButton);
    }
}
