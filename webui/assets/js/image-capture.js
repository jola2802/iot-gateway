// Image Capture Prozessverwaltung JavaScript
let processes = [];
let devices = [];

// Lade Bilder vom Server und aktualisiere die Anzeige
async function loadImagesFiles() {
    const imagesFilesContainer = document.getElementById('images-files-container');
    if (!imagesFilesContainer) return;

    try {
        const response = await fetch('/api/images');
        if (!response.ok) {
            throw new Error('Fehler beim Laden der Bilder');
        }
        
        const data = await response.json();
        
        // Sortiere Bilder nach Timestamp (neueste zuerst)
        data.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));

        // Erstelle ein responsives Grid für die Bilder
        const rowDiv = document.createElement('div');
        rowDiv.classList.add('row', 'row-cols-1', 'row-cols-md-3', 'row-cols-lg-5', 'g-3');

        // Zeige maximal 25 Bilder an
        const imagesToShow = data.slice(0, 50);
        
        imagesToShow.forEach((image) => {
            const colDiv = document.createElement('div');
            colDiv.classList.add('col');
            
            const cardDiv = document.createElement('div');
            cardDiv.classList.add('card', 'h-100');
            
            // Bild aus dem Base64-String anzeigen
            const imgElement = document.createElement('img');
            imgElement.src = image.image.startsWith('data:image') ? image.image : 'data:image/png;base64,' + image.image;
            imgElement.alt = 'Bild von ' + image.device;
            imgElement.classList.add('card-img-top', 'img-thumbnail');
            imgElement.style.height = '150px';
            imgElement.style.objectFit = 'cover';
            
            // Klickbar machen für Vollbildansicht
            imgElement.style.cursor = 'pointer';
            imgElement.addEventListener('click', () => {
                previewImage(image.image.replace('data:image/png;base64,', '').replace('data:image/jpeg;base64,', ''), image.timestamp);
            });
            
            // Karteninhalt
            const cardBody = document.createElement('div');
            cardBody.classList.add('card-body', 'p-2');
            
            // Prozess-ID + Gerätenamen anzeigen
            const deviceName = document.createElement('h6');
            deviceName.classList.add('card-title');
            deviceName.textContent = `${image.device_name} (Process ID: ${image.process_id}) (${image.id})`;
            
            // Timestamp anzeigen
            const timestamp = document.createElement('small');
            timestamp.classList.add('text-muted');
            timestamp.textContent = formatDateTime(image.timestamp);
            
            cardBody.appendChild(deviceName);
            cardBody.appendChild(timestamp);
            
            cardDiv.appendChild(imgElement);
            cardDiv.appendChild(cardBody);
            colDiv.appendChild(cardDiv);
            rowDiv.appendChild(colDiv);
        });
        
        // Container leeren und neue Bilder einfügen
        imagesFilesContainer.innerHTML = '';
        imagesFilesContainer.appendChild(rowDiv);
        
        if (imagesToShow.length === 0) {
            imagesFilesContainer.innerHTML = '<p class="text-center text-muted">Keine Bilder verfügbar</p>';
        }
        
    } catch (error) {
        console.error('Fehler beim Laden der Bilder:', error);
        imagesFilesContainer.innerHTML = '<p class="text-center text-danger">Fehler beim Laden der Bilder</p>';
    }
}

// Initialisiere Bilder-Anzeige beim ersten Laden
function initializeImagesFiles() {
    const downloadAllImagesBtn = document.getElementById('download-all-images');
    
    // Download-Button Event Listener nur einmal einrichten
    if (downloadAllImagesBtn && !downloadAllImagesBtn.hasAttribute('data-initialized')) {
        downloadAllImagesBtn.setAttribute('data-initialized', 'true');
        downloadAllImagesBtn.addEventListener('click', () => {
            fetch('/api/images/download')
                .then(response => response.blob())
                .then(blob => {
                    const url = window.URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.style.display = 'none';
                    a.href = url;
                    a.download = 'all_images.zip';
                    document.body.appendChild(a);
                    a.click();
                    window.URL.revokeObjectURL(url);
                    document.body.removeChild(a);
                })
                .catch(error => {
                    console.error('Fehler beim Download:', error);
                    showAlert('Fehler beim Download der Bilder', 'danger');
                });
        });
    }
    
    // Erste Ladung der Bilder
    loadImagesFiles();
}


// Funktion zum Anzeigen eines Bildes im Modal
function showImageModal(image) {
    let modal = document.getElementById('image-preview-modal');
    
    if (!modal) {
        modal = document.createElement('div');
        modal.id = 'image-preview-modal';
        modal.classList.add('modal', 'fade');
        modal.setAttribute('tabindex', '-1');
        modal.setAttribute('role', 'dialog');
        modal.setAttribute('aria-hidden', 'true');
        
        modal.innerHTML = `
            <div class="modal-dialog modal-lg">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">Image Preview</h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
                    </div>
                    <div class="modal-body text-center">
                        <img id="modal-image" class="img-fluid" alt="Image Preview">
                        <div class="mt-2">
                            <p id="modal-device" class="mb-1"></p>
                            <p id="modal-timestamp" class="text-muted small"></p>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Close</button>
                        <a id="modal-download" class="btn btn-primary" download>
                            <i class="fas fa-download"></i> Download
                        </a>
                    </div>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
    }
    
    // Setze die Bildinformationen
    const modalImage = document.getElementById('modal-image');
    const modalDevice = document.getElementById('modal-device');
    const modalTimestamp = document.getElementById('modal-timestamp');
    const modalDownload = document.getElementById('modal-download');
    
    const imageSource = image.image.startsWith('data:image') ? image.image : 'data:image/png;base64,' + image.image;
    
    modalImage.src = imageSource;
    modalDevice.textContent = 'Device ID: ' + image.device;
    
    const date = new Date(image.timestamp);
    modalTimestamp.textContent = 'Captured at: ' + date.toLocaleString('de-DE');
    
    modalDownload.href = imageSource;
    modalDownload.download = `image_${image.device}_${date.toISOString().split('T')[0]}.png`;
    
    const bsModal = new bootstrap.Modal(modal);
    bsModal.show();
}


// Initialisierung beim Laden der Seite
document.addEventListener('DOMContentLoaded', function() {
    loadDevices();
    loadProcesses(); 
    setupEventListeners();
    initializeImagesFiles();
});

// // Funktionen auch direkt aufrufen, damit sie sofort ausgeführt werden
// loadDevices();
// loadProcesses();
// setupEventListeners();

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
    // console.log('Loading processes...');
    try {
        const response = await fetch('/api/image-capture-processes');
        // console.log('Response status:', response.status);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const data = await response.json();
        // console.log('Received data:', data);
        
        processes = data.processes || [];
        // console.log('Processes loaded:', processes.length);
        
        renderProcessTable();
    } catch (error) {
        console.error('Fehler beim Laden der Prozesse:', error);
        showAlert('Fehler beim Laden der Prozesse: ' + error.message, 'danger');
        
        // Fallback: Leere Tabelle anzeigen
        processes = [];
        renderProcessTable();
    }
}

// Prozessliste rendern
function renderProcessTable() {
    // console.log('Rendering process table with', processes.length, 'processes');
    const tbody = document.getElementById('processTableBody');
    
    if (!tbody) {
        console.error('Process table body not found!');
        return;
    }
    
    tbody.innerHTML = '';
    
    // Statistiken berechnen
    updateProcessStatistics();
    
    if (processes.length === 0) {
        // console.log('No processes found, showing empty message');
        tbody.innerHTML = '<tr><td colspan="10" class="text-center">No processes found</td></tr>';
        return;
    }
    
    processes.forEach(process => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${escapeHtml(process.id)}</td>
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
                    '<span class="text-muted">Manual</span>'
                }
            </td>
            <td>${formatDateTime(process.last_execution)}</td>
            <td>
                ${process.last_image ? 
                    `<button class="btn btn-sm btn-outline-primary" onclick="previewImage('${process.last_image}', '${process.last_execution}')">
                        <i class="fas fa-eye"></i> Preview
                    </button>` : 
                    '<span class="text-muted">No image</span>'
                }
            </td>
            <td>
                ${getUploadStatusBadge(process.last_upload_status, process.last_upload_error)}
                ${process.enable_upload ? 
                    `<br><small class="text-muted">Success: ${process.upload_success_count} | Failed: ${process.upload_failure_count}</small>` : 
                    '<br><small class="text-muted">Upload disabled</small>'
                }
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
        case 'paused': return 'bg-warning';
        default: return 'bg-light text-dark';
    }
}

// Status Text ermitteln
function getStatusText(status) {
    switch (status) {
        case 'running': return 'Running';
        case 'stopped': return 'Stopped';
        case 'error': return 'Error';
        case 'paused': return 'Paused';
        default: return status || 'Unknown';
    }
}

// Upload Status Badge erstellen
function getUploadStatusBadge(status, error) {
    switch (status) {
        case 'success':
            return '<span class="badge bg-success">Upload Success</span>';
        case 'failed':
            return `<span class="badge bg-danger" title="${escapeHtml(error || 'Upload failed')}">Upload Failed</span>`;
        case 'skipped':
            return '<span class="badge bg-warning">Upload Skipped</span>';
        case 'not_attempted':
        default:
            return '<span class="badge bg-secondary">No Upload</span>';
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
    // console.log('Saving process...');
    const formData = getFormData();
    // console.log('Form data:', formData);
    
    if (!formData.name || !formData.device_id) {
        showAlert('Please fill in all required fields', 'warning');
        return;
    }
    
    try {
        // console.log('Sending POST request to /api/image-capture-processes');
        const response = await fetch('/api/image-capture-processes', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(formData)
        });
        
        // console.log('Save response status:', response.status);
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Error saving process');
        }
        
        const result = await response.json();
        // console.log('Save result:', result);
        showAlert('Process created successfully', 'success');
        
        // Neuen Prozess zur lokalen Liste hinzufügen
        if (result.process) {
            // console.log('Adding new process to list');
            processes.unshift(result.process); // Am Anfang hinzufügen
            renderProcessTable();
        }
        
        // Modal schließen und Formular zurücksetzen
        const modal = bootstrap.Modal.getInstance(document.getElementById('addProcessModal'));
        modal.hide();
        document.getElementById('processForm').reset();
        
        // Upload Headers zurücksetzen
        populateUploadHeaders({});
        
        // Nach dem Speichern nochmal die Prozesse laden
        // console.log('Reloading processes after save');
        await loadProcesses();
        
    } catch (error) {
        console.error('Error saving:', error);
        showAlert('Error saving: ' + error.message, 'danger');
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
    // Method Args aus Textarea parsen
    let methodArgs = {};
    const methodArgsText = document.getElementById('methodArgs').value.trim();
    if (methodArgsText) {
        try {
            methodArgs = JSON.parse(methodArgsText);
        } catch (e) {
            console.warn('Invalid JSON in method args, using empty object:', e);
            showAlert('Warning: Invalid JSON in Method Arguments. Using empty arguments instead.', 'warning');
            methodArgs = {};
        }
    }

    return {
        name: document.getElementById('processName').value,
        device_id: parseInt(document.getElementById('deviceSelect').value),
        endpoint: document.getElementById('endpoint').value,
        object_id: document.getElementById('objectId').value,
        method_id: document.getElementById('methodId').value,
        method_args: methodArgs,
        check_node_id: document.getElementById('checkNodeId').value,
        image_node_id: document.getElementById('imageNodeId').value,
        ack_node_id: document.getElementById('ackNodeId').value,
        enable_upload: document.getElementById('enableUpload').checked,
        upload_url: document.getElementById('uploadUrl').value,
        upload_headers: collectUploadHeaders(),
        timestamp_header_name: document.getElementById('timestampHeaderName').value,
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
        
        showAlert('Process started successfully', 'success');
        loadProcesses();
        
    } catch (error) {
        console.error('Error starting:', error);
        showAlert('Error starting: ' + error.message, 'danger');
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
            throw new Error(error.error || 'Error stopping');
        }
        
        showAlert('Process stopped successfully', 'success');
        loadProcesses();
        
    } catch (error) {
        console.error('Error stopping:', error);
        showAlert('Error stopping: ' + error.message, 'danger');
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
            throw new Error(error.error || 'Error executing');
        }
        
        showAlert('Image Capture executed', 'success');
        
        // Nach kurzer Zeit die Prozesse neu laden, um das neue Bild zu sehen
        setTimeout(() => {
            loadProcesses();
        }, 4000);
        
    } catch (error) {
        console.error('Error executing:', error);
        showAlert('Error executing: ' + error.message, 'danger');
    }
}

// Prozess bearbeiten
async function editProcess(processId) {
    const process = processes.find(p => p.id === processId);
    if (!process) {
        showAlert('Process not found', 'danger');
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
    
    // Method Args füllen
    if (process.method_args && Object.keys(process.method_args).length > 0) {
        document.getElementById('methodArgs').value = JSON.stringify(process.method_args, null, 2);
    } else {
        document.getElementById('methodArgs').value = '';
    }
    
    // Timestamp Header Name füllen
    document.getElementById('timestampHeaderName').value = process.timestamp_header_name || '';
    
    // Upload Headers füllen
    populateUploadHeaders(process.upload_headers || {});
    
    // Modal-Titel ändern
    document.querySelector('#addProcessModal .modal-title').textContent = 'Edit process';
    
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
            throw new Error(error.error || 'Error updating');
        }
        
        showAlert('Process updated successfully', 'success');
        
        // Modal schließen
        const modal = bootstrap.Modal.getInstance(document.getElementById('addProcessModal'));
        modal.hide();
        
        // Formular zurücksetzen und Button zurücksetzen
        document.getElementById('processForm').reset();
        document.getElementById('saveProcess').onclick = saveProcess;
        document.querySelector('#addProcessModal .modal-title').textContent = 'New Image Capture Process';
        
        // Upload Headers zurücksetzen
        populateUploadHeaders({});
        
        // Prozesse neu laden
        loadProcesses();
        
    } catch (error) {
        console.error('Error updating:', error);
        showAlert('Error updating: ' + error.message, 'danger');
    }
}

// Prozess löschen
async function deleteProcess(processId) {
    if (!confirm('Are you sure you want to delete this process?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/image-capture-processes/${processId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Error deleting');
        }
        
        showAlert('Process deleted successfully', 'success');
        loadProcesses();
        
    } catch (error) {
        console.error('Error deleting:', error);
        showAlert('Error deleting: ' + error.message, 'danger');
    }
}

// Bildvorschau anzeigen
function previewImage(base64Image, timestamp) {
    if (!base64Image) {
        showAlert('No image available', 'warning');
        return;
    }
    
    const imageSrc = `data:image/jpeg;base64,${base64Image}`;
    document.getElementById('previewImage').src = imageSrc;
    document.getElementById('downloadImage').href = imageSrc;
    
    // Timestamp anzeigen
    const timestampElement = document.getElementById('imageTimestamp');
    if (timestamp && timestamp !== 'null') {
        timestampElement.textContent = `Captured at: ${formatDateTime(timestamp)}`;
    } else {
        timestampElement.textContent = 'Timestamp not available';
    }
    
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

// Prozess-Statistiken aktualisieren
function updateProcessStatistics() {
    const stats = {
        running: 0,
        stopped: 0,
        error: 0,
        total: processes.length
    };
    
    processes.forEach(process => {
        switch (process.status) {
            case 'running':
                stats.running++;
                break;
            case 'stopped':
                stats.stopped++;
                break;
            case 'error':
                stats.error++;
                break;
        }
    });
    
    // Statistiken in der UI aktualisieren
    document.getElementById('runningCount').textContent = stats.running;
    document.getElementById('stoppedCount').textContent = stats.stopped;
    document.getElementById('errorCount').textContent = stats.error;
    document.getElementById('totalCount').textContent = stats.total;
}

// Node-RED Endpoint-Informationen anzeigen
function showEndpointInfo(processId, processName) {
    // Modal-Inhalt mit Endpoint-Informationen füllen
    document.getElementById('endpointProcessName').textContent = processName;
    document.getElementById('executeEndpoint').textContent = `POST /api/image-capture-processes/${processId}/trigger`;
    document.getElementById('startEndpoint').textContent = `POST /api/image-capture-processes/${processId}/start-external`;
    document.getElementById('stopEndpoint').textContent = `POST /api/image-capture-processes/${processId}/stop-external`;
    document.getElementById('statusEndpoint').textContent = `GET /api/image-capture-processes/${processId}`;
    
    // Modal anzeigen
    const modal = new bootstrap.Modal(document.getElementById('endpointInfoModal'));
    modal.show();
}

// Automatisches Neuladen der Prozesse und Bilder alle 10 Sekunden
setInterval(async () => {
    await loadProcesses();
    await new Promise(resolve => setTimeout(resolve, 500)); // Warte 0.5 Sekunde
    await loadImagesFiles();
}, 7000);
