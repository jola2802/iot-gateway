// ==================== KONSTANTEN ====================
const DEVICE_TYPES = {
    OPC_UA: 'opc-ua',
    S7: 's7',
    MQTT: 'mqtt'
};

// ==================== GRUNDLEGENDE HILFSFUNKTIONEN ====================
function hideAllConfigs() {
    const configIds = ['opc-ua-config', 's7-config', 'mqtt-config'];
    configIds.forEach(id => {
        const config = document.getElementById(id);
        if (config) config.style.display = 'none';
    });
}

// ==================== DEVICE MODAL FUNKTIONEN ====================
function initializeNewDeviceModal() {
    const selectDeviceType = document.getElementById('select-device-type');

    function showConfig(selectedType) {
        hideAllConfigs();
        const config = document.getElementById(`${selectedType}-config`);
        if (config) {
            config.style.display = 'block';
            if (selectedType === DEVICE_TYPES.OPC_UA) {
                initializeOpcUaSecuritySettings('');
            }
        }
    }

    hideAllConfigs();
    const initialSelection = selectDeviceType.value;
    showConfig(initialSelection);

    const newDeviceModal = document.getElementById('modal-new-device');
    if (newDeviceModal) {
        selectDeviceType.removeEventListener('change', handleDeviceTypeChange);
    }

    function handleDeviceTypeChange() {
        showConfig(selectDeviceType.value);
    }

    selectDeviceType.addEventListener('change', handleDeviceTypeChange);
}

// ==================== OPC UA CREDENTIALS HANDLING ====================
function handleOpcUaCredentials(prefix = '') {
    const containerName = 'opc-ua-credentials' + prefix;
    const container = document.getElementById(containerName);
    if (!container) {
        console.error(`Container ${containerName} nicht gefunden`);
        return;
    }

    const ids = {
        select: 'select-authentication-settings' + prefix,
        usernameGroup: 'username-group' + prefix,
        passwordGroup: 'password-group' + prefix,
        username: 'username' + prefix,
        password: 'password' + prefix
    };

    if (!document.getElementById(ids.usernameGroup)) {
        container.innerHTML = `
            <div id="${ids.usernameGroup}" class="form-group mb-3" style="display: none;">
                <label class="form-label" for="${ids.username}"><strong>Username</strong></label>
                <input type="text" class="form-control" id="${ids.username}" placeholder="Enter username">
            </div>
            <div id="${ids.passwordGroup}" class="form-group mb-3" style="display: none;">
                <label class="form-label" for="${ids.password}"><strong>Password</strong></label>
                <input type="password" class="form-control" id="${ids.password}" placeholder="Enter password">
            </div>
        `;
    }

    const selectAuth = document.getElementById(ids.select);
    if (!selectAuth) {
        console.error(`Select element ${ids.select} nicht gefunden`);
        return;
    }

    function toggleCredentialsFields() {
        const usernameGroup = document.getElementById(ids.usernameGroup);
        const passwordGroup = document.getElementById(ids.passwordGroup);
        const display = selectAuth.value === 'user-pw' ? 'block' : 'none';
        
        if (usernameGroup) usernameGroup.style.display = display;
        if (passwordGroup) passwordGroup.style.display = display;
    }

    if (!selectAuth.dataset.listenerBound) {
        selectAuth.addEventListener('change', toggleCredentialsFields);
        selectAuth.dataset.listenerBound = 'true';
    }

    toggleCredentialsFields();
    return ids;
}

function initializeOpcUaSecuritySettings(prefix = '') {
    return handleOpcUaCredentials(prefix);
}

async function initializeEditDeviceModal(device_id) {
    return new Promise(async (resolve, reject) => {
        try {
            // Prüfen ob device_id gültig ist
            if (!device_id) {
                throw new Error('Keine gültige Device ID');
            }

            // Speichere device_id im localStorage für spätere Verwendung
            localStorage.setItem('device_id', device_id);
            
            // Hole die Referenz zum select Element
            const selectDeviceType = document.getElementById('select-device-type-1');
            if (!selectDeviceType) {
                throw new Error('select-device-type-1 Element nicht gefunden');
            }
            
            const response = await fetch(`/api/getDevice/${device_id}`);
            if (!response.ok) {
                throw new Error(`Failed to fetch device details: ${response.status}`);
            }
            const deviceDataArr = await response.json();

            const deviceData = deviceDataArr.device; 

            console.log('Device Data:', deviceData);

            // Gerätedaten in das Modal einfügen
            document.getElementById('device-name-1').value = deviceData.deviceName || '';
            
            // Setze den Device Type und deaktiviere das Feld
            selectDeviceType.value = deviceData.deviceType || 'opc-ua';
            selectDeviceType.disabled = true;

            // Deaktiviere das Namensfeld
            document.getElementById('device-name-1').disabled = true;

            // Zeige die entsprechende Konfiguration
            const configIds = ['opc-ua-config-1', 's7-config-1', 'mqtt-config-1'];
            configIds.forEach(id => {
                const config = document.getElementById(id);
                if (config) {
                    config.style.display = id.includes(deviceData.deviceType) ? 'block' : 'none';
                }
            });

            if (deviceData.deviceType === 'opc-ua') {
                // Clear old values
                document.getElementById('address-1').value = '';
                document.getElementById('select-security-policy-1').value = 'none';
                document.getElementById('select-authentication-settings-1').value = 'anonymous';
                document.getElementById('select-security-mode-1').value = 'None';

                // Fill new values
                document.getElementById('address-1').value = deviceData.address || '';
                document.getElementById('select-security-policy-1').value = deviceData.securityPolicy.String || 'none';
                document.getElementById('select-authentication-settings-1').value = deviceData.authentication || 'anonymous';
                document.getElementById('select-security-mode-1').value = deviceData.securityMode.String || 'None';

                // Credentials verwalten
                const credentialIds = handleOpcUaCredentials('-1');
                
                if (deviceData.username.String !== '' && deviceData.password.String !== '') {
                    document.getElementById(credentialIds.select).value = 'user-pw';
                    document.getElementById(credentialIds.username).value = deviceData.username.String;
                    document.getElementById(credentialIds.password).value = deviceData.password.String;
                } else {
                    document.getElementById(credentialIds.select).value = 'anonymous';
                }
                
                // Trigger change event to update visibility
                document.getElementById(credentialIds.select).dispatchEvent(new Event('change'));

                document.getElementById('acquisition-time-opc-ua-1').value = deviceData.acquisitionTime;
                
            } else if (deviceData.deviceType === 's7') {
                document.querySelector('#s7-config-1 [placeholder="192.168.2.100:102"]').value = deviceData.address || '';
                document.querySelector('#s7-config-1 [placeholder="0"]').value = deviceData.rack.String || '';
                document.querySelector('#s7-config-1 [placeholder="1"]').value = deviceData.slot.String || '';
                document.getElementById('acquisition-time-2').value = deviceData.acquisitionTime;
            } else if (deviceData.deviceType === 'mqtt') {
                document.querySelector('#mqtt-config-1 [placeholder="Type in password"]').value = deviceData.password.String || '';
            }

            // Tabelle der Datenpunkte leeren
            const datapointsTableBody = document.querySelector('#ipi-table tbody');
            datapointsTableBody.innerHTML = '';

            if (deviceData.datapoint) {
                // Tabelle für Datenpunkte
                const datapointsTableBody = document.querySelector('#ipi-table tbody');
                datapointsTableBody.innerHTML = '';

                deviceData.datapoint.forEach(datapoint => {
                    const row = document.createElement('tr');
                    row.setAttribute('datapoint-id', datapoint.datapointId);

                    const idCell = document.createElement('td');
                    idCell.textContent = datapoint.datapointId;
                    idCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(idCell);

                    const nameCell = document.createElement('td');
                    nameCell.textContent = datapoint.name;
                    nameCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(nameCell);

                    const datatypeCell = document.createElement('td');
                    datatypeCell.textContent = datapoint.datatype || 'INT';
                    row.appendChild(datatypeCell);

                    const addressCell = document.createElement('td');
                    addressCell.textContent = datapoint.address || '';
                    addressCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(addressCell);

                    const actionCell = document.createElement('td');
                    actionCell.innerHTML = `
                    <a href="#" class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" 
                        style="margin-left: 5px;" 
                        onclick="confirmDeleteDatapoint('${datapoint.datapointId}', event)">
                            <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                        </a>
                    `;
                    row.appendChild(actionCell);

                    datapointsTableBody.appendChild(row);
                });

            }
            datapointsTableBody.appendChild(createEmptyRow(deviceData.deviceType));

            // Am Ende der Funktion:
            resolve();
        } catch (error) {
            console.error(`Error in initializeEditDeviceModal: ${error.message}`);
            reject(error);
        }
    });
}

// ==================== DATENPUNKT HANDLING ====================
function createEmptyRow(deviceType) {
    const emptyRow = document.createElement('tr');
    const cells = {
        id: createInputCell('Enter ID'),
        name: createInputCell('Enter Name'),
        datatype: createDatatypeCell(deviceType),
        address: createInputCell('Enter Address / Node ID'),
        action: createActionCell()
    };

    Object.values(cells).forEach(cell => emptyRow.appendChild(cell));
    return emptyRow;
}

function createInputCell(placeholder) {
    const cell = document.createElement('td');
    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'form-control';
    input.placeholder = placeholder;
    cell.appendChild(input);
    return cell;
}

function createDatatypeCell(deviceType) {
    const cell = document.createElement('td');
    
    if (deviceType === DEVICE_TYPES.OPC_UA) {
        cell.textContent = 'N/A';
        
        // Verstecktes Eingabefeld für Konsistenz
        const hiddenInput = document.createElement('input');
        hiddenInput.type = 'hidden';
        hiddenInput.value = 'N/A';
        cell.appendChild(hiddenInput);
        
        return cell;
    }
    
    const datatypeSelect = document.createElement('select');
    datatypeSelect.className = 'form-select';
    ['INT', 'REAL', 'BOOL', 'STRING'].forEach(type => {
        const option = document.createElement('option');
        option.value = type;
        option.textContent = type;
        datatypeSelect.appendChild(option);
    });
    cell.appendChild(datatypeSelect);
    return cell;
}

function createActionCell() {
    const cell = document.createElement('td');
    const saveButton = document.createElement('button');
    saveButton.type = 'button';
    saveButton.className = 'btn btn-success';
    saveButton.textContent = 'Save';
    saveButton.addEventListener('click', function() {
        const row = this.closest('tr');
        const inputs = row.querySelectorAll('input, select');
        const [idInput, nameInput, datatypeInput, addressInput] = inputs;
        
        const deviceType = document.getElementById('select-device-type').value || 
                          document.getElementById('select-device-type-1').value;
        
        saveDatapoint(
            idInput.value,
            nameInput.value,
            datatypeInput?.value || 'N/A',
            addressInput.value,
            deviceType
        );
    });
    cell.appendChild(saveButton);
    return cell;
}

function saveDatapoint(id, name, datatype, address, deviceType) {
    if (!isValidDatapoint(name, address, datatype, deviceType)) {
        alert('Please fill all fields before saving!');
        return;
    }

    const datapointId = id || Math.floor(Math.random() * 1000);
    const newRow = createSavedDatapointRow(datapointId, name, datatype, address);
    
    const tableBody = document.querySelector('#ipi-table tbody');
    if (!tableBody) {
        console.error('Tabelle nicht gefunden');
        return;
    }

    // Wenn es eine leere Zeile gibt, füge die neue Zeile davor ein
    const emptyRow = tableBody.querySelector('tr:last-child');
    if (emptyRow) {
        tableBody.insertBefore(newRow, emptyRow);
        clearInputRow(emptyRow);
    } else {
        // Falls keine leere Zeile existiert, füge die neue Zeile hinzu und erstelle eine neue leere Zeile
        tableBody.appendChild(newRow);
        tableBody.appendChild(createEmptyRow(deviceType));
    }
}

function isValidDatapoint(name, address, datatype, deviceType) {
    if (!name || !address) return false;
    if (deviceType === DEVICE_TYPES.S7 && !datatype) return false;
    return true;
}

function createSavedDatapointRow(id, name, datatype, address) {
    const row = document.createElement('tr');
    row.setAttribute('datapoint-id', id);

    const createCell = (text) => {
        const cell = document.createElement('td');
        cell.textContent = text;
        cell.style.color = 'rgb(121, 121, 121)';
        return cell;
    };

    [id, name, datatype, address].forEach(text => 
        row.appendChild(createCell(text))
    );

    const actionCell = document.createElement('td');
    actionCell.innerHTML = `
        <a href="#" class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" 
            style="margin-left: 5px;" 
            onclick="confirmDeleteDatapoint('${id}', event)">
            <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
        </a>
    `;
    row.appendChild(actionCell);

    return row;
}

function clearInputRow(row) {
    if (!row) return;
    const inputs = row.querySelectorAll('input, select');
    inputs.forEach(input => input.value = '');
}

// ==================== LÖSCHFUNKTIONEN ====================
function confirmDeleteDatapoint(datapointId, event) {
    if (event) {
        event.preventDefault();
    }
    
    if (confirm(`Möchten Sie den Datapoint mit der ID ${datapointId} wirklich löschen?`)) {
        const row = document.querySelector(`tr[datapoint-id="${datapointId}"]`); 
        if (row) {
            row.remove();
        }
    }
}

function confirmDeleteDevice(deviceId) {
    if (confirm(`Möchten Sie das Gerät mit der ID ${deviceId} wirklich löschen?`)) {
        fetch(`/api/delete-device/${deviceId}`, { method: 'DELETE' })
            .then(response => {
                if (!response.ok) {
                    throw new Error(`Fehler beim Löschen (HTTP ${response.status}).`);
                }
                alert('Gerät erfolgreich gelöscht!');
                location.reload();
            })
            .catch(error => {
                console.error('Fehler beim Löschen des Geräts:', error);
                alert('Ein unbekannter Fehler ist aufgetreten.');
            });
    }
}