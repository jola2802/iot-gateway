// Gemeinsame Variablen für alle Module
window.dataForwarding = {
    elements: {},
    state: {
        lupeButtons: document.querySelectorAll('.btn-open-browsed-nodes')
    }
};

// Füge diese Konstante am Anfang der Datei hinzu
const NODES_PER_PAGE = 10;

// Am Anfang der Datei nach den Importen
async function loadRouteData(routeId) {
    try {
        const response = await fetch(`/api/route/${routeId}`);
        if (!response.ok) {
            throw new Error(`Error fetching route data: ${response.status}`);
        }
        
        const routeData = await response.json();
        
        // Modal öffnen und Daten setzen
        const modal = new bootstrap.Modal(document.getElementById('modal-new-route'));
        modal.show();
        
        // Setze die Formulardaten
        document.getElementById('select-destination-type').value = routeData.destinationType;
        
        // Rest der Logik bleibt in modals.js
        if (window.dataForwarding.functions.setRouteData) {
            window.dataForwarding.functions.setRouteData(routeData);
        }
    } catch (error) {
        console.error('Error loading route data:', error);
        alert('Error loading route data: ' + error.message);
    }
}

// Hauptfunktion zum Laden der Daten
async function fetchAndPopulateDataForwarding() {
    try {
        const response = await fetch(`/api/get-routes`);
        if (!response.ok) {
            throw new Error(`Error fetching /api/get-routes: ${response.status}`);
        }

        const dataForwarding = await response.json();
        const tableBody = document.querySelector('#table-data-forwarding tbody');
        tableBody.innerHTML = ''; // Tabelle leeren

        // Prüfe ob dataForwarding null oder undefined ist
        if (!dataForwarding || dataForwarding.length === 0) {
            tableBody.innerHTML = '<tr><td colspan="6" class="text-center">No data forwarding routes found</td></tr>';
            return;
        }

        dataForwarding.forEach(route => {
            const row = document.createElement('tr');

            // Route ID
            const routeIdCell = document.createElement('td');
            routeIdCell.textContent = route.id;
            row.appendChild(routeIdCell);

            // Type
            const typeCell = document.createElement('td');
            typeCell.textContent = route.destinationType;
            row.appendChild(typeCell);

            // Devices
            const devicesCell = document.createElement('td');
            devicesCell.textContent = route.devices;
            // remove the first and last character
            devicesCell.textContent = devicesCell.textContent.slice(1, -1);
            // show the devices in a list
            devicesCell.innerHTML = `<ul>${devicesCell.textContent.split(',').map(device => `<li>${device}</li>`).join('')}</ul>`;
            row.appendChild(devicesCell);

            // Address
            const addressCell = document.createElement('td');
            addressCell.textContent = route.destination_url || route.filePath;
            row.appendChild(addressCell);

            // Last Send
            const lastSendCell = document.createElement('td');
            lastSendCell.textContent = route.last_send || 'N/A';
            row.appendChild(lastSendCell);

            // Actions
            const actionsCell = document.createElement('td');
            actionsCell.innerHTML = `
                <a class="btn btnMaterial btn-flat success semicircle" href="#">
                    <i class="fas fa-pen" data-route-id="${route.id}" data-bs-toggle="modal" data-bs-target="#modal-new-route"></i>
                </a>
                <a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" style="margin-left: 5px;" href="#">
                    <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                </a>
            `;

            // Event-Listener für den Bearbeiten-Button
            const editBtn = actionsCell.querySelector('.fa-pen');
            editBtn.addEventListener('click', (e) => {
                e.preventDefault();
                loadRouteData(route.id);
            });

            // Event-Listener für das Löschen
            const trashBtn = actionsCell.querySelector('.fa-trash').parentElement;
            trashBtn.addEventListener('click', async (e) => {
                e.preventDefault();
                if (confirm('Möchten Sie diese Route wirklich löschen?')) {
                    try {
                        const response = await fetch(`/api/route/${route.id}`, {
                            method: 'DELETE'
                        });
                        if (!response.ok) throw new Error('Fehler beim Löschen');
                        await fetchAndPopulateDataForwarding(); // Tabelle neu laden
                    } catch (error) {
                        console.error('Fehler beim Löschen:', error);
                        alert('Fehler beim Löschen der Route');
                    }
                }
            });

            row.appendChild(actionsCell);
            tableBody.appendChild(row);
        });
    } catch (error) {
        console.error('Error fetching /api/get-routes:', error);
        alert('Fehler beim Laden der Routen');
    }
}


// Event-Listener für das Dokument
document.addEventListener('DOMContentLoaded', () => {
    // Initialisiere die DOM-Elemente
    window.dataForwarding.elements = {
        destinationTypeSelect: document.getElementById('select-destination-type'),
        restConfig: document.getElementById('rest-config'),
        fileConfig: document.getElementById('file-config'),
        mqttConfig: document.getElementById('mqtt-config'),
        dataForwardingTable: document.getElementById('table-data-forwarding'),
        imageProcessTable: document.getElementById('table-image-capture-process')
    };

    loadNodeRedURLFlow();
    initializeImagesFiles();

    // Initialisiere den Event-Listener für destinationTypeSelect
    const { destinationTypeSelect } = window.dataForwarding.elements;
    if (destinationTypeSelect) {
        destinationTypeSelect.addEventListener('change', () => {
            const selectedValue = destinationTypeSelect.value;
            hideAllConfigsRoute();
            
            switch (selectedValue) {
                case 'rest':
                    if (restConfig) restConfig.style.display = 'block';
                    break;
                case 'file-based':
                    if (fileConfig) fileConfig.style.display = 'block';
                    break;
                case 'mqtt':
                    if (mqttConfig) mqttConfig.style.display = 'block';
                    break;
            }
        });
    }

    // Rest der Initialisierung...
    // fetchAndPopulateDataForwarding();
    fetchAndPopulateImgProcesses();
    initializeLupeButtons();
});

// Exportiere die benötigten Funktionen
window.dataForwarding.functions = {
    // fetchAndPopulateDataForwarding,
    fetchAndPopulateImgProcesses,
    loadRouteData,
    deleteRoute
};

async function loadNodeRedURLFlow() {

    fetch('/api/get-node-red-url')
        .then(response => response.json())
        .then(data => {
            const nodeRedURLFlow = document.getElementById('node-red-data-forwarding-flow');
            nodeRedURLFlow.src = data.nodeRedURL;
        });
}

async function fetchAndPopulateDevicesProcess() {
    try {
        const response = await fetch('/api/list-opc-ua-devices');
        if (!response.ok) {
            throw new Error(`Error fetching devices: ${response.status}`);
        }
        const data = await response.json();
        const select = document.getElementById('select-opc-device');
        
        if (select) {
            select.innerHTML = '<option value="" selected disabled>Please select a device...</option>';
            if (data.devices && Array.isArray(data.devices)) {
                data.devices.forEach(device => {
                    const option = document.createElement('option');
                    option.value = device;
                    option.textContent = device;
                    select.appendChild(option);
                });
            }
        }
    } catch (error) {
        console.error('Error fetching devices:', error);
    }
}

// ##################
// Devices for Image Capture
// ##################

// ************************
// Browse for node
// ************************

// Aktualisiere die Initialisierung der Lupe-Buttons
function initializeLupeButtons() {
    const lupeButtons = document.querySelectorAll('.btn-open-browsed-nodes');
    
    lupeButtons.forEach(button => {
        button.addEventListener('click', async function(event) {
            event.preventDefault();
            
            // Hole das zugehörige Input-Feld
            const inputField = button.closest('.input-group').querySelector('input');
            if (!inputField) {
                console.error('Kein Input-Feld gefunden');
                return;
            }
            
            // Hole die ausgewählte Device ID
            const deviceSelect = document.getElementById('select-opc-device');
            const selectedDevice = deviceSelect?.value;
            
            if (!selectedDevice) {
                alert('Bitte wählen Sie zuerst ein Gerät aus');
                return;
            }

            // Zeige das Modal direkt mit Spinner
            const browsedNodesModal = new bootstrap.Modal(document.getElementById('modal-browsed-nodes'));
            const modalBody = document.getElementById('modal-browsed-nodes').querySelector('.modal-body');
            
            // Zeige Spinner
            modalBody.innerHTML = `
                <div class="text-center p-4">
                    <div class="spinner-border text-primary" role="status">
                        <span class="visually-hidden">Lade Nodes...</span>
                    </div>
                    <div class="mt-2">Lade verfügbare Nodes...</div>
                </div>
            `;
            
            browsedNodesModal.show();

            try {
                const response = await fetch(`/api/browseNodes/${selectedDevice}`);
                if (!response.ok) throw new Error('Fehler beim Laden der Nodes');
                
                const data = await response.json();
                
                // Prüfe das Format der Daten
                const nodes = Array.isArray(data) ? data : (data.nodes || []);
                
                if (nodes.length === 0) {
                    modalBody.innerHTML = `
                        <div class="text-center p-4">
                            <div class="text-muted">Keine Nodes gefunden</div>
                        </div>
                    `;
                    return;
                }

                // Erstelle Container für Nodes und Pagination
                modalBody.innerHTML = `
                    <div class="list-group mb-3"></div>
                    <nav aria-label="Node navigation" class="d-flex justify-content-between align-items-center">
                        <span class="text-muted">
                            Zeige <span id="showing-nodes">0-0</span> von ${nodes.length} Nodes
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

                const listGroup = modalBody.querySelector('.list-group');
                const prevButton = modalBody.querySelector('#prev-page');
                const nextButton = modalBody.querySelector('#next-page');
                const showingNodesSpan = modalBody.querySelector('#showing-nodes');
                
                let currentPage = 0;
                const totalPages = Math.ceil(nodes.length / NODES_PER_PAGE);

                // Funktion zum Anzeigen der aktuellen Seite
                function showPage(page) {
                    const start = page * NODES_PER_PAGE;
                    const end = Math.min(start + NODES_PER_PAGE, nodes.length);
                    const pageNodes = nodes.slice(start, end);
                    
                    // Update Anzeige-Info
                    showingNodesSpan.textContent = `${start + 1}-${end}`;
                    
                    // Update Button-Status
                    prevButton.disabled = page === 0;
                    nextButton.disabled = page >= totalPages - 1;
                    
                    // Leere und fülle die Liste
                    listGroup.innerHTML = '';
                    pageNodes.forEach(node => {
                        const nodeInfo = parseNodeString(node);
                        const listItem = document.createElement('div');
                        listItem.className = 'list-group-item';
                        
                        listItem.innerHTML = `
                            <div class="d-flex justify-content-between align-items-center">
                                <div>
                                    <strong>${nodeInfo.id}</strong>
                                    ${nodeInfo.description ? `<br><small class="text-muted">${nodeInfo.description}</small>` : ''}
                                </div>
                                <button class="btn btn-primary btn-sm select-node" 
                                        data-node-id="${nodeInfo.id}">
                                    Auswählen
                                </button>
                            </div>
                        `;
                        
                        const selectBtn = listItem.querySelector('.select-node');
                        selectBtn.addEventListener('click', () => {
                            inputField.value = nodeInfo.id;
                            browsedNodesModal.hide();
                        });
                        
                        listGroup.appendChild(listItem);
                    });
                }

                // Event-Listener für Pagination
                prevButton.addEventListener('click', () => {
                    if (currentPage > 0) {
                        currentPage--;
                        showPage(currentPage);
                    }
                });

                nextButton.addEventListener('click', () => {
                    if (currentPage < totalPages - 1) {
                        currentPage++;
                        showPage(currentPage);
                    }
                });

                // Zeige erste Seite
                showPage(0);
                
            } catch (error) {
                console.error('Fehler beim Laden der Nodes:', error);
                modalBody.innerHTML = `
                    <div class="text-center p-4">
                        <div class="text-danger">
                            <i class="fas fa-exclamation-circle"></i>
                            Fehler beim Laden der Nodes: ${error.message}
                        </div>
                    </div>
                `;
            }
        });
    });
}

// Aktualisiere die parseNodeString Funktion
function parseNodeString(node) {
    // Wenn node ein Objekt mit der erwarteten Struktur ist
    if (node && typeof node === 'object' && node.NodeID) {
        return {
            id: node.NodeID,
            nodeClass: node.NodeClass?.toString() || '',
            name: node.BrowseName || '',
            description: node.Path || ''
        };
    }
    
    // Fallback für unbekannte Formate
    return {
        id: 'unknown',
        nodeClass: '',
        name: '',
        description: 'Ungültiges Node-Format'
    };
}

// Aktualisiere die updateBrowsedNodesModal Funktion
function updateBrowsedNodesModal(data) {
    const browsedNodesModalElement = document.getElementById('modal-browsed-nodes');
    const listGroup = browsedNodesModalElement.querySelector('.list-group');

    if (!listGroup) {
        console.error("Element mit Klasse 'list-group' im Modal nicht gefunden.");
        return;
    }

    // Leeren der bestehenden Liste
    listGroup.innerHTML = '';

    // Iterieren über die Nodes und Hinzufügen zur Liste
    data.nodes.forEach(node => {
        const listItem = document.createElement('div');
        listItem.className = 'list-group-item';
        listItem.innerHTML = `
            <div class="d-flex justify-content-between align-items-center">
                <span>${node.name}</span>
                <button class="btn btn-primary btn-sm select-node" 
                        data-node-id="${node.id}" 
                        data-node-name="${node.name}">
                    Auswählen
                </button>
            </div>
        `;
        listGroup.appendChild(listItem);
    });
}

// Exportiere die benötigten Funktionen
window.dataForwarding.functions = {
    ...window.dataForwarding.functions,
    // fetchAndPopulateDataForwarding,
    fetchAndPopulateImgProcesses,
    loadRouteData
};

// Füge diese Funktion vor dem Export hinzu
async function deleteRoute(routeId) {
    try {
        const response = await fetch(`/api/route/${routeId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        await fetchAndPopulateDataForwarding(); // Tabelle neu laden
        alert('Route erfolgreich gelöscht!');
    } catch (error) {
        console.error('Fehler beim Löschen der Route:', error);
        alert('Fehler beim Löschen der Route: ' + error.message);
    }
}

function initializeNodeBrowser(deviceId) {
    const modalBody = document.querySelector('#node-browser-modal .modal-body');
    modalBody.innerHTML = `
        <div class="mb-3">
            <input type="text" class="form-control" id="node-search" 
                   placeholder="Suche nach Nodes..." style="margin-bottom: 15px;">
        </div>
        <div class="list-group mb-3"></div>
        <nav aria-label="Node navigation" class="d-flex justify-content-between align-items-center">
            <span class="text-muted">
                Zeige <span id="showing-nodes">0-0</span> von <span id="total-nodes">0</span> Nodes
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

    function displayNodes(nodes) {
        const listGroup = document.querySelector('#node-browser-modal .list-group');
        listGroup.innerHTML = '';
        
        const start = currentPage * nodesPerPage;
        const end = Math.min(start + nodesPerPage, nodes.length);
        document.getElementById('showing-nodes').textContent = `${start + 1}-${end}`;
        
        nodes.forEach(node => {
            const listItem = document.createElement('div');
            listItem.className = 'list-group-item py-2'; // Reduzierte Polsterung
            
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
                        <input type="radio" name="node-selection" class="form-check-input" 
                            value="${node.NodeID}">
                    </div>
                </div>
            `;
            
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
}
