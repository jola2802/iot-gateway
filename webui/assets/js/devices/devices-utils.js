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

// Neue Funktionen für Node-Browser
function showBrowseNodesButton(deviceType) {
    const browseButton = document.getElementById('browse-nodes-btn');
    if (browseButton) {
        browseButton.style.display = deviceType === 'opc-ua' ? 'block' : 'none';
    }
}

let selectedNodes = new Set();
let currentNodes = [];
let filteredNodes = [];
let currentPage = 0;
const nodesPerPage = 20;

async function initializeNodeBrowser() {
    const deviceId = localStorage.getItem('device_id');
    const browseButton = document.getElementById('browse-nodes-btn');
    const nodeBrowserModal = new bootstrap.Modal(document.getElementById('node-browser-modal'));
    
    // Füge Suchfeld zum Modal hinzu
    const modalBody = document.querySelector('#node-browser-modal .modal-body');
    modalBody.innerHTML = `
        <div class="mb-3">
            <input type="text" class="form-control" id="node-search" 
                   placeholder="Search for Nodes..." style="margin-bottom: 15px;">
        </div>
        <div class="list-group mb-3"></div>
        <nav aria-label="Node navigation" class="d-flex justify-content-between align-items-center">
            <span class="text-muted">
                Show <span id="showing-nodes">0-0</span> of <span id="total-nodes">0</span> Nodes
            </span>
            <ul class="pagination mb-0">
                <li class="page-item">
                    <button class="page-link" id="prev-page" aria-label="Previous">
                        <span aria-hidden="true">&laquo;</span>
                    </button>
                </li>
                <li class="page-item">
                    <button class="page-link" id="next-page" aria-label="Next">
                        <span aria-hidden="true">&raquo;</span>
                    </button>
                </li>
            </ul>
        </nav>
    `;

    let currentNodes = [];
    let filteredNodes = [];
    let currentPage = 0;
    const nodesPerPage = 20;

    function showLoadingState() {
        const listGroup = document.querySelector('#node-browser-modal .list-group');
        listGroup.innerHTML = `
            <div class="text-center py-5">
                <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Loading Nodes...</span>
                </div>
                <div class="mt-2 text-muted">Loading available Nodes...</div>
            </div>
        `;
        document.getElementById('node-search').disabled = true;
        document.getElementById('prev-page').disabled = true;
        document.getElementById('next-page').disabled = true;
        document.getElementById('showing-nodes').textContent = '0-0';
        document.getElementById('total-nodes').textContent = '0';
    }

    // Event Listener für die Suche
    document.getElementById('node-search').addEventListener('input', (e) => {
        const searchTerm = e.target.value.toLowerCase();
        filteredNodes = currentNodes.filter(node => 
            node.NodeID.toLowerCase().includes(searchTerm) ||
            node.BrowseName.toLowerCase().includes(searchTerm) ||
            (node.Path && node.Path.toLowerCase().includes(searchTerm))
        );
        currentPage = 0;
        displayNodes(filteredNodes.slice(0, nodesPerPage));
        setupPagination(filteredNodes);
    });

    browseButton.addEventListener('click', async () => {
        nodeBrowserModal.show();
        showLoadingState();
        
        try {
            const response = await fetch(`/api/browseNodes/${deviceId}`);
            if (!response.ok) throw new Error('Fehler beim Laden der Nodes');
            
            const data = await response.json();
            currentNodes = data.nodes || [];
            filteredNodes = [...currentNodes];
            
            // Sortiere die Nodes alphabetisch nach NodeID
            currentNodes.sort((a, b) => a.NodeID.localeCompare(b.NodeID));
            
            document.getElementById('total-nodes').textContent = currentNodes.length;
            document.getElementById('node-search').disabled = false;
            selectedNodes.clear();
            currentPage = 0;
            displayNodes(currentNodes.slice(0, nodesPerPage));
            setupPagination(currentNodes);
        } catch (error) {
            console.error('Fehler:', error);
            displayErrorInModal(nodeBrowserModal._element, 'Fehler beim Laden der Nodes: ' + error.message);
        }
    });

    function displayNodes(nodes) {
        const listGroup = document.querySelector('#node-browser-modal .list-group');
        listGroup.innerHTML = '';
        
        const start = currentPage * nodesPerPage;
        const end = Math.min(start + nodesPerPage, nodes.length);
        document.getElementById('showing-nodes').textContent = `${start + 1}-${end}`;
        
        nodes.forEach(node => {
            const listItem = document.createElement('div');
            listItem.className = 'list-group-item py-2';
            
            listItem.innerHTML = `
                <div class="d-flex justify-content-between align-items-center">
                    <div class="text-truncate" style="max-width: 90%;">
                        <div class="d-flex align-items-center">
                            <strong class="me-2" style="font-size: 0.9rem;">${node.NodeID}</strong>
                            <small class="text-muted" style="font-size: 0.85rem;">
                                ${node.BrowseName}
                            </small>
                        </div>
                        ${node.Path ? `
                            <small class="text-muted d-block text-truncate" style="font-size: 0.8rem;">
                                ${node.Path}
                            </small>
                        ` : ''}
                    </div>
                    <div class="form-check ms-2">
                        <input type="checkbox" class="form-check-input" 
                            ${selectedNodes.has(node.NodeID) ? 'checked' : ''}>
                    </div>
                </div>
            `;
            
            const checkbox = listItem.querySelector('input[type="checkbox"]');
            checkbox.addEventListener('change', () => {
                if (checkbox.checked) {
                    selectedNodes.add(node.NodeID);
                } else {
                    selectedNodes.delete(node.NodeID);
                }
            });
            
            listGroup.appendChild(listItem);
        });
    }

    function setupPagination(nodes) {
        const prevButton = document.getElementById('prev-page');
        const nextButton = document.getElementById('next-page');
        const pageCount = Math.ceil(nodes.length / nodesPerPage);
        
        prevButton.disabled = currentPage === 0;
        nextButton.disabled = currentPage >= pageCount - 1;
        
        prevButton.onclick = () => {
            if (currentPage > 0) {
                currentPage--;
                displayNodes(nodes.slice(currentPage * nodesPerPage, (currentPage + 1) * nodesPerPage));
                setupPagination(nodes);
            }
        };
        
        nextButton.onclick = () => {
            if (currentPage < pageCount - 1) {
                currentPage++;
                displayNodes(nodes.slice(currentPage * nodesPerPage, (currentPage + 1) * nodesPerPage));
                setupPagination(nodes);
            }
        };
    }

    document.getElementById('save-selected-nodes').addEventListener('click', () => {
        addSelectedNodesToTable();
        nodeBrowserModal.hide();
    });
}

// Hilfsfunktion zum Anzeigen von Fehlermeldungen
function displayErrorInModal(modalElement, message) {
    const modalBody = modalElement.querySelector('.modal-body');
    if (!modalBody) return;

    modalBody.innerHTML = `
        <div class="alert alert-danger" role="alert">
            <i class="fas fa-exclamation-circle"></i>
            ${message}
        </div>
    `;
}

function addSelectedNodesToTable() {
    const datapointsTable = document.querySelector('#ipi-table tbody');
    
    selectedNodes.forEach(nodeId => {
        const datapointId = Math.floor(10000 + Math.random() * 90000).toString(); // Random 5 digit number
        // use only last 3 parts of nodeId
        const nodeIdParts = nodeId.split(';').pop().replace(/[^a-zA-Z0-9]/g, '_').split('_').slice(-3);
        const lastThreeParts = nodeIdParts.join('_'); 
        const row = createSavedDatapointRow(
            datapointId,
            `${lastThreeParts}`,
            'N/A',
            nodeId
        );
        
        const emptyRow = datapointsTable.querySelector('tr:last-child');
        if (emptyRow) {
            datapointsTable.insertBefore(row, emptyRow);
        } else {
            datapointsTable.appendChild(row);
        }
    });
}

async function initializeEditDeviceModal(device_id) {
    return new Promise(async (resolve, reject) => {
        try {
            if (!device_id) {
                throw new Error('Keine gültige Device ID');
            }

            localStorage.setItem('device_id', device_id);
            
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

            document.getElementById('device-name-1').value = deviceData.deviceName || '';
            
            selectDeviceType.value = deviceData.deviceType || 'opc-ua';
            selectDeviceType.disabled = true;

            document.getElementById('device-name-1').disabled = true;

            const configIds = ['opc-ua-config-1', 's7-config-1', 'mqtt-config-1'];
            configIds.forEach(id => {
                const config = document.getElementById(id);
                if (config) {
                    config.style.display = id.includes(deviceData.deviceType) ? 'block' : 'none';
                }
            });

            if (deviceData.deviceType === 'opc-ua') {
                document.getElementById('address-1').value = '';
                document.getElementById('select-security-policy-1').value = 'none';
                document.getElementById('select-authentication-settings-1').value = 'anonymous';
                document.getElementById('select-security-mode-1').value = 'None';

                document.getElementById('address-1').value = deviceData.address || '';
                document.getElementById('select-security-policy-1').value = deviceData.securityPolicy.String || 'none';
                document.getElementById('select-authentication-settings-1').value = deviceData.authentication || 'anonymous';
                document.getElementById('select-security-mode-1').value = deviceData.securityMode.String || 'None';

                const credentialIds = handleOpcUaCredentials('-1');
                
                if (deviceData.username.String !== '' && deviceData.password.String !== '') {
                    document.getElementById(credentialIds.select).value = 'user-pw';
                    document.getElementById(credentialIds.username).value = deviceData.username.String;
                    document.getElementById(credentialIds.password).value = deviceData.password.String;
                } else {
                    document.getElementById(credentialIds.select).value = 'anonymous';
                }
                
                document.getElementById(credentialIds.select).dispatchEvent(new Event('change'));

                document.getElementById('acquisition-time-opc-ua-1').value = deviceData.acquisitionTime;
                
            } else if (deviceData.deviceType === 's7') {
                document.querySelector('#address-2').value = deviceData.address || '';
                document.querySelector('#rack').value = deviceData.rack.String || '';
                document.querySelector('#slot').value = deviceData.slot.String || '';
                document.getElementById('acquisition-time-2').value = deviceData.acquisitionTime;
            } else if (deviceData.deviceType === 'mqtt') {
                document.querySelector('#mqtt-config-1 [placeholder="Type in password"]').value = deviceData.password.String || '';
            }

            const datapointsTableBody = document.querySelector('#ipi-table tbody');
            datapointsTableBody.innerHTML = '';

            if (deviceData.datapoint) {
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

            showBrowseNodesButton(deviceData.deviceType);
            initializeNodeBrowser();

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
        
        const hiddenInput = document.createElement('input');
        hiddenInput.type = 'hidden';
        hiddenInput.value = 'N/A';
        cell.appendChild(hiddenInput);
        
        return cell;
    }
    
    const datatypeSelect = document.createElement('select');
    datatypeSelect.className = 'form-select';
    ['-', 'BOOL', 'INT', 'DINT', 'REAL', 'WORD', 'DWORD', 'STRING'].forEach(type => {
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

    const emptyRow = tableBody.querySelector('tr:last-child');
    if (emptyRow) {
        tableBody.insertBefore(newRow, emptyRow);
        clearInputRow(emptyRow);
    } else {
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
    
    if (confirm(`Are you sure you want to delete the datapoint with the ID ${datapointId}?`)) {
        const row = document.querySelector(`tr[datapoint-id="${datapointId}"]`); 
        if (row) {
            row.remove();
        }
    }
}

function confirmDeleteDevice(deviceId) {
    if (confirm(`Are you sure you want to delete the device with the ID ${deviceId}?`)) {
        fetch(`/api/delete-device/${deviceId}`, { method: 'DELETE' })
            .then(response => {
                if (!response.ok) {
                    throw new Error(`Fehler beim Löschen (HTTP ${response.status}).`);
                }
                location.reload();
            })
            .catch(error => {
                console.error('Fehler beim Löschen des Geräts:', error);
                alert('Ein unbekannter Fehler ist aufgetreten.');
            });
    }
}