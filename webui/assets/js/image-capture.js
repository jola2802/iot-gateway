// Image Capture Prozessverwaltung JavaScript
let processes = [];
let devices = [];

// Initialisierung beim Laden der Seite
document.addEventListener('DOMContentLoaded', function() {
    loadDevices();
    loadProcesses();
    setupEventListeners();
});

// Event Listener einrichten
function setupEventListeners() {
    // Speichern Button
    document.getElementById('saveProcess').addEventListener('click', saveProcess);
    
    // Geräteauswahl ändert sich
    document.getElementById('deviceSelect').addEventListener('change', function() {
        const selectedDevice = devices.find(d => d.id == this.value);
        if (selectedDevice) {
            document.getElementById('endpoint').value = selectedDevice.address;
        }
    });
    
    // Upload Headers hinzufügen
    document.getElementById('addHeader').addEventListener('click', addHeaderRow);
    
    // Upload Headers Container Event Delegation
    document.getElementById('uploadHeadersContainer').addEventListener('click', function(e) {
        if (e.target.classList.contains('remove-header')) {
            e.target.closest('.header-row').remove();
        }
    });
}

// Geräte laden
async function loadDevices() {
    try {
        const response = await fetch('/api/getDevices');
        if (!response.ok) throw new Error('Fehler beim Laden der Geräte');
        
        const data = await response.json();
        devices = data.devices || [];
        populateDeviceSelect();
    } catch (error) {
        console.error('Fehler beim Laden der Geräte:', error);
        showAlert('Fehler beim Laden der Geräte', 'danger');
    }
}

// Geräteauswahl füllen
function populateDeviceSelect() {
    const select = document.getElementById('deviceSelect');
    select.innerHTML = '<option value="">Gerät auswählen...</option>';
    
    devices.forEach(device => {
        const option = document.createElement('option');
        option.value = device.id;
        option.textContent = device.deviceName;
        select.appendChild(option);
    });
}

// Prozesse laden
async function loadProcesses() {
    try {
        const response = await fetch('/api/image-capture-processes');
        if (!response.ok) throw new Error('Fehler beim Laden der Prozesse');
        
        const data = await response.json();
        processes = data.processes || [];
        renderProcessTable();
    } catch (error) {
        console.error('Fehler beim Laden der Prozesse:', error);
        showAlert('Fehler beim Laden der Prozesse', 'danger');
    }
}

// Prozessliste rendern
function renderProcessTable() {
    const tbody = document.getElementById('processTableBody');
    tbody.innerHTML = '';
    
    if (processes.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="text-center">Keine Prozesse gefunden</td></tr>';
        return;
    }
    
    processes.forEach(process => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${escapeHtml(process.name)}</td>
            <td>${escapeHtml(process.device_name || 'Unbekannt')}</td>
            <td>
                <span class="badge ${getStatusBadgeClass(process.status)}">
                    ${getStatusText(process.status)}
                </span>
            </td>
            <td>
                ${process.enable_cyclic ? 
                    `<span class="badge bg-info">${process.cyclic_interval}s</span>` : 
                    '<span class="text-muted">Manuell</span>'
                }
            </td>
            <td>${formatDateTime(process.last_execution)}</td>
            <td>
                ${process.last_image ? 
                    `<button class="btn btn-sm btn-outline-primary" onclick="previewImage('${process.last_image}')">
                        <i class="fas fa-eye"></i> Vorschau
                    </button>` : 
                    '<span class="text-muted">Kein Bild</span>'
                }
            </td>
            <td>
                <button class="btn btn-sm btn-outline-info" onclick="showEndpointInfo(${process.id}, '${escapeHtml(process.name)}')">
                    <i class="fas fa-code"></i> API Endpoints
                </button>
            </td>
            <td>
                <div class="btn-group" role="group">
                    ${process.status === 'running' ? 
                        `<a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" onclick="stopProcess(${process.id})" title="Prozess stoppen" style="margin-left: 5px;">
                            <i class="fas fa-stop btnNoBorders" style="color: #FFC107;"></i>
                        </a>` :
                        `<a class="btn btnMaterial btn-flat success semicircle" onclick="startProcess(${process.id})" title="Prozess starten">
                            <i class="fas fa-play"></i>
                        </a>`
                    }
                    <a class="btn btnMaterial btn-flat info semicircle" onclick="executeProcess(${process.id})" title="Einmalig ausführen" style="margin-left: 5px;">
                        <i class="fas fa-camera"></i>
                    </a>
                    <a class="btn btnMaterial btn-flat success semicircle" onclick="editProcess(${process.id})" title="Prozess bearbeiten" style="margin-left: 5px;">
                        <i class="fas fa-pen"></i>
                    </a>
                    <a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" onclick="deleteProcess(${process.id})" title="Prozess löschen" style="margin-left: 5px;">
                        <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                    </a>
                </div>
            </td>
        `;
        tbody.appendChild(row);
    });
}

// Status Badge Klasse ermitteln
function getStatusBadgeClass(status) {
    switch (status) {
        case 'running': return 'bg-success';
        case 'stopped': return 'bg-secondary';
        case 'error': return 'bg-danger';
        default: return 'bg-secondary';
    }
}

// Status Text ermitteln
function getStatusText(status) {
    switch (status) {
        case 'running': return 'Läuft';
        case 'stopped': return 'Gestoppt';
        case 'error': return 'Fehler';
        default: return 'Unbekannt';
    }
}

// Datum/Zeit formatieren
function formatDateTime(dateTime) {
    if (!dateTime) return '-';
    try {
        return new Date(dateTime).toLocaleString('de-DE');
    } catch (error) {
        return dateTime;
    }
}

// HTML Escaping
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Prozess speichern
async function saveProcess() {
    const formData = getFormData();
    
    if (!formData.name || !formData.device_id) {
        showAlert('Bitte füllen Sie alle Pflichtfelder aus', 'warning');
        return;
    }
    
    try {
        const response = await fetch('/api/image-capture-processes', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(formData)
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Fehler beim Speichern');
        }
        
        const result = await response.json();
        showAlert('Prozess erfolgreich erstellt', 'success');
        
        // Neuen Prozess zur lokalen Liste hinzufügen
        if (result.process) {
            processes.unshift(result.process); // Am Anfang hinzufügen
            renderProcessTable();
        }
        
        // Modal schließen und Formular zurücksetzen
        const modal = bootstrap.Modal.getInstance(document.getElementById('addProcessModal'));
        modal.hide();
        document.getElementById('processForm').reset();
        
        // Upload Headers zurücksetzen
        populateUploadHeaders({});
        
    } catch (error) {
        console.error('Fehler beim Speichern:', error);
        showAlert('Fehler beim Speichern: ' + error.message, 'danger');
    }
}

// Upload Header Zeile hinzufügen
function addHeaderRow() {
    const container = document.getElementById('uploadHeadersContainer');
    const headerRow = document.createElement('div');
    headerRow.className = 'header-row mb-2';
    headerRow.innerHTML = `
        <div class="row">
            <div class="col-5">
                <input type="text" class="form-control header-key" placeholder="Header Name">
            </div>
            <div class="col-5">
                <input type="text" class="form-control header-value" placeholder="Header Value">
            </div>
            <div class="col-2">
                <button type="button" class="btn btn-sm btn-danger remove-header">
                    <i class="fas fa-trash"></i>
                </button>
            </div>
        </div>
    `;
    container.appendChild(headerRow);
}

// Upload Headers aus DOM sammeln
function collectUploadHeaders() {
    const headers = {};
    const headerRows = document.querySelectorAll('.header-row');
    
    headerRows.forEach(row => {
        const keyInput = row.querySelector('.header-key');
        const valueInput = row.querySelector('.header-value');
        
        if (keyInput && valueInput && keyInput.value.trim() && valueInput.value.trim()) {
            headers[keyInput.value.trim()] = valueInput.value.trim();
        }
    });
    
    return headers;
}

// Formulardaten sammeln
function getFormData() {
    return {
        name: document.getElementById('processName').value,
        device_id: parseInt(document.getElementById('deviceSelect').value),
        endpoint: document.getElementById('endpoint').value,
        object_id: document.getElementById('objectId').value,
        method_id: document.getElementById('methodId').value,
        method_args: {},
        check_node_id: document.getElementById('checkNodeId').value,
        image_node_id: document.getElementById('imageNodeId').value,
        ack_node_id: document.getElementById('ackNodeId').value,
        enable_upload: document.getElementById('enableUpload').checked,
        upload_url: document.getElementById('uploadUrl').value,
        upload_headers: collectUploadHeaders(),
        enable_cyclic: document.getElementById('enableCyclic').checked,
        cyclic_interval: parseInt(document.getElementById('cyclicInterval').value) || 30,
        description: document.getElementById('processDescription').value
    };
}

// Prozess starten
async function startProcess(processId) {
    try {
        const response = await fetch(`/api/image-capture-processes/${processId}/start`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Fehler beim Starten');
        }
        
        showAlert('Prozess erfolgreich gestartet', 'success');
        loadProcesses();
        
    } catch (error) {
        console.error('Fehler beim Starten:', error);
        showAlert('Fehler beim Starten: ' + error.message, 'danger');
    }
}

// Prozess stoppen
async function stopProcess(processId) {
    try {
        const response = await fetch(`/api/image-capture-processes/${processId}/stop`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Fehler beim Stoppen');
        }
        
        showAlert('Prozess erfolgreich gestoppt', 'success');
        loadProcesses();
        
    } catch (error) {
        console.error('Fehler beim Stoppen:', error);
        showAlert('Fehler beim Stoppen: ' + error.message, 'danger');
    }
}

// Prozess einmalig ausführen
async function executeProcess(processId) {
    try {
        const response = await fetch(`/api/image-capture-processes/${processId}/execute`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Fehler bei der Ausführung');
        }
        
        showAlert('Image Capture gestartet', 'success');
        
        // Nach kurzer Zeit die Prozesse neu laden, um das neue Bild zu sehen
        setTimeout(() => {
            loadProcesses();
        }, 2000);
        
    } catch (error) {
        console.error('Fehler bei der Ausführung:', error);
        showAlert('Fehler bei der Ausführung: ' + error.message, 'danger');
    }
}

// Prozess bearbeiten
async function editProcess(processId) {
    const process = processes.find(p => p.id === processId);
    if (!process) {
        showAlert('Prozess nicht gefunden', 'danger');
        return;
    }
    
    // Formular mit Prozessdaten füllen
    document.getElementById('processName').value = process.name;
    document.getElementById('deviceSelect').value = process.device_id;
    document.getElementById('endpoint').value = process.endpoint;
    document.getElementById('objectId').value = process.object_id;
    document.getElementById('methodId').value = process.method_id;
    document.getElementById('checkNodeId').value = process.check_node_id;
    document.getElementById('imageNodeId').value = process.image_node_id;
    document.getElementById('ackNodeId').value = process.ack_node_id;
    document.getElementById('enableUpload').checked = process.enable_upload;
    document.getElementById('uploadUrl').value = process.upload_url;
    document.getElementById('enableCyclic').checked = process.enable_cyclic;
    document.getElementById('cyclicInterval').value = process.cyclic_interval || 30;
    document.getElementById('processDescription').value = process.description || '';
    
    // Upload Headers füllen
    populateUploadHeaders(process.upload_headers || {});
    
    // Modal-Titel ändern
    document.querySelector('#addProcessModal .modal-title').textContent = 'Prozess bearbeiten';
    
    // Speichern-Button für Update konfigurieren
    const saveButton = document.getElementById('saveProcess');
    saveButton.onclick = () => updateProcess(processId);
    
    // Modal öffnen
    const modal = new bootstrap.Modal(document.getElementById('addProcessModal'));
    modal.show();
}

// Upload Headers in DOM füllen
function populateUploadHeaders(headers) {
    const container = document.getElementById('uploadHeadersContainer');
    container.innerHTML = '';
    
    if (Object.keys(headers).length === 0) {
        // Standard-Header-Zeile hinzufügen
        addHeaderRow();
    } else {
        Object.entries(headers).forEach(([key, value]) => {
            const headerRow = document.createElement('div');
            headerRow.className = 'header-row mb-2';
            headerRow.innerHTML = `
                <div class="row">
                    <div class="col-5">
                        <input type="text" class="form-control header-key" placeholder="Header Name" value="${escapeHtml(key)}">
                    </div>
                    <div class="col-5">
                        <input type="text" class="form-control header-value" placeholder="Header Value" value="${escapeHtml(value)}">
                    </div>
                    <div class="col-2">
                        <button type="button" class="btn btn-sm btn-danger remove-header">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </div>
            `;
            container.appendChild(headerRow);
        });
    }
}

// Prozess aktualisieren
async function updateProcess(processId) {
    const formData = getFormData();
    formData.id = processId;
    
    try {
        const response = await fetch(`/api/image-capture-processes/${processId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(formData)
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Fehler beim Aktualisieren');
        }
        
        showAlert('Prozess erfolgreich aktualisiert', 'success');
        
        // Modal schließen
        const modal = bootstrap.Modal.getInstance(document.getElementById('addProcessModal'));
        modal.hide();
        
        // Formular zurücksetzen und Button zurücksetzen
        document.getElementById('processForm').reset();
        document.getElementById('saveProcess').onclick = saveProcess;
        document.querySelector('#addProcessModal .modal-title').textContent = 'Neuer Image Capture Prozess';
        
        // Upload Headers zurücksetzen
        populateUploadHeaders({});
        
        // Prozesse neu laden
        loadProcesses();
        
    } catch (error) {
        console.error('Fehler beim Aktualisieren:', error);
        showAlert('Fehler beim Aktualisieren: ' + error.message, 'danger');
    }
}

// Prozess löschen
async function deleteProcess(processId) {
    if (!confirm('Sind Sie sicher, dass Sie diesen Prozess löschen möchten?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/image-capture-processes/${processId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Fehler beim Löschen');
        }
        
        showAlert('Prozess erfolgreich gelöscht', 'success');
        loadProcesses();
        
    } catch (error) {
        console.error('Fehler beim Löschen:', error);
        showAlert('Fehler beim Löschen: ' + error.message, 'danger');
    }
}

// Bildvorschau anzeigen
function previewImage(base64Image) {
    if (!base64Image) {
        showAlert('Kein Bild verfügbar', 'warning');
        return;
    }
    
    const imageSrc = `data:image/jpeg;base64,${base64Image}`;
    document.getElementById('previewImage').src = imageSrc;
    document.getElementById('downloadImage').href = imageSrc;
    
    const modal = new bootstrap.Modal(document.getElementById('imagePreviewModal'));
    modal.show();
}

// Alert anzeigen
function showAlert(message, type = 'info') {
    // Bestehende Alerts entfernen
    const existingAlerts = document.querySelectorAll('.alert');
    existingAlerts.forEach(alert => alert.remove());
    
    // Neuen Alert erstellen
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
    alertDiv.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    // Alert am Anfang des Containers einfügen
    const container = document.querySelector('.container-fluid');
    container.insertBefore(alertDiv, container.firstChild);
    
    // Alert nach 5 Sekunden automatisch ausblenden
    setTimeout(() => {
        if (alertDiv.parentNode) {
            alertDiv.remove();
        }
    }, 5000);
}

// Node-RED Endpoint-Informationen anzeigen
function showEndpointInfo(processId, processName) {
    // Modal-Inhalt mit Endpoint-Informationen füllen
    document.getElementById('endpointProcessName').textContent = processName;
    document.getElementById('executeEndpoint').textContent = `POST /api/image-capture-processes/${processId}/execute`;
    document.getElementById('startEndpoint').textContent = `POST /api/image-capture-processes/${processId}/start`;
    document.getElementById('stopEndpoint').textContent = `POST /api/image-capture-processes/${processId}/stop`;
    document.getElementById('statusEndpoint').textContent = `GET /api/image-capture-processes/${processId}`;
    
    // Modal anzeigen
    const modal = new bootstrap.Modal(document.getElementById('endpointInfoModal'));
    modal.show();
}

// Automatisches Neuladen der Prozesse alle 30 Sekunden
setInterval(() => {
    loadProcesses();
}, 30000);
