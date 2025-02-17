// Globale Variablen für den Wizard
let currentStep = 1;
const totalSteps = 6;
let processData = {
    device: '',
    methodParentNode: '',
    methodImageNode: '',
    imageNode: '',
    captureCompletedNode: '',
    readCompletedNode: '',
    captureMode: 'interval',
    interval: '',
    triggerNode: '',
    restUri: '',
    headers: []
};

// Initialisierung des Wizards
document.addEventListener('DOMContentLoaded', function() {
    initializeProcessModal();
});

// Funktion zur Initialisierung des Process-Modals
function initializeProcessModal() {
    const processModal = document.getElementById('modal-new-img-process');
    if (!processModal) {
        console.error('Process Modal nicht gefunden');
        return;
    }

    // Initialisiere den Wizard
    initializeWizard();
    setupEventListeners();

    // Event-Listener für das Modal
    processModal.addEventListener('show.bs.modal', async function(event) {
        try {
            await fetchAndPopulateDevicesProcess();
            resetWizard();
        } catch (error) {
            console.error('Fehler bei der Modal-Initialisierung:', error);
            event.preventDefault(); // Verhindere das Öffnen des Modals bei Fehlern
            alert('Fehler beim Laden der OPC UA Geräte: ' + error.message);
        }
    });
}

// Wizard Initialisierung
function initializeWizard() {
    updateStepVisibility();
    updateButtonStates();
    setupCaptureMode();
}

// Event Listener Setup
function setupEventListeners() {
    // Navigation Buttons im Image-Process-Modal
    document.getElementById('btn-next').addEventListener('click', nextStep);
    document.getElementById('btn-prev').addEventListener('click', prevStep);
    document.getElementById('btn-save').addEventListener('click', saveProcess);

    // Capture Mode Change im Image-Process-Modal
    document.getElementById('capture-mode').addEventListener('change', function(e) {
        processData.captureMode = e.target.value;
        toggleCaptureOptions();
    });

    // Header Hinzufügen im Image-Process-Modal
    document.getElementById('add-header-button-p').addEventListener('click', addHeader);
}

// Geräte laden
async function loadDevices() {
    try {
        const response = await fetch('/api/get-devices-for-routes');
        if (!response.ok) throw new Error('Fehler beim Laden der Geräte');
        
        const data = await response.json();
        const select = document.getElementById('select-device');
        select.innerHTML = '<option value="" selected disabled>Please select a device...</option>';
        
        data.devices.forEach(device => {
            const option = document.createElement('option');
            option.value = device;
            option.textContent = device;
            select.appendChild(option);
        });
    } catch (error) {
        console.error('Fehler beim Laden der Geräte:', error);
        alert('Fehler beim Laden der Geräte: ' + error.message);
    }
}

// Navigation
function nextStep() {
    if (validateCurrentStep()) {
        currentStep++;
        if (currentStep === totalSteps) {
            updateReviewContent();
        }
        updateStepVisibility();
        updateButtonStates();
        updateProgressBar();
    }
}

function prevStep() {
    if (currentStep > 1) {
        currentStep--;
        updateStepVisibility();
        updateButtonStates();
        updateProgressBar();
    }
}

// Validierung
function validateCurrentStep() {
    switch(currentStep) {
        case 1:
            return validateDeviceStep();
        case 2:
            return validateMethodsStep();
        case 3:
            return validateNodesStep();
        case 4:
            return validateCaptureStep();
        case 5:
            return validateRestStep();
        default:
            return true;
    }
}

// Einzelne Validierungsfunktionen
function validateDeviceStep() {
    const device = document.getElementById('select-opc-device').value;
    if (!device) {
        alert('Please select an OPC UA device.');
        return false;
    }
    processData.device = device;
    return true;
}

function validateMethodsStep() {
    const parentNode = document.getElementById('method-parent-node-input').value;
    const imageNode = document.getElementById('method-image-node-input').value;
    
    if (!parentNode || !imageNode) {
        alert('Please fill in all method nodes.');
        return false;
    }
    
    processData.methodParentNode = parentNode;
    processData.methodImageNode = imageNode;
    return true;
}

// Validierungsfunktionen für alle Schritte
function validateNodesStep() {
    const imageNode = document.getElementById('image-node-input').value;
    const captureCompletedNode = document.getElementById('capture-completed-node-input').value;
    const readCompletedNode = document.getElementById('read-completed-node-input').value;
    
    if (!imageNode || !captureCompletedNode || !readCompletedNode) {
        alert('Please fill in all node fields.');
        return false;
    }
    
    processData.imageNode = imageNode;
    processData.captureCompletedNode = captureCompletedNode;
    processData.readCompletedNode = readCompletedNode;
    return true;
}

function validateCaptureStep() {
    const captureMode = document.getElementById('capture-mode').value;
    processData.captureMode = captureMode;
    
    if (captureMode === 'interval') {
        const interval = document.getElementById('interval-input').value;
        if (!interval || interval < 1) {
            alert('Please enter a valid interval (at least 1 second).');
            return false;
        }
        processData.interval = parseInt(interval);
        processData.triggerNode = '';
    } else {
        const triggerNode = document.getElementById('trigger-node-input').value;
        if (!triggerNode) {
            alert('Please enter a trigger node.');
            return false;
        }
        processData.triggerNode = triggerNode;
        processData.interval = 0;
    }
    return true;
}

function validateRestStep() {
    const restUri = document.getElementById('rest-uri-input').value;
    if (!restUri) {
        alert('Please enter a REST-URI.');
        return false;
    }
    processData.restUri = restUri;
    // Headers wurden bereits durch addHeader() zu processData.headers hinzugefügt
    return true;
}

// Funktion zum Validieren aller Schritte
function validateAllSteps() {
    const validationFunctions = [
        validateDeviceStep,
        validateMethodsStep,
        validateNodesStep,
        validateCaptureStep,
        validateRestStep
    ];

    for (let i = 0; i < validationFunctions.length; i++) {
        if (!validationFunctions[i]()) {
            currentStep = i + 1;
            updateStepVisibility();
            updateButtonStates();
            updateProgressBar();
            return false;
        }
    }
    return true;
}

// UI Updates
function updateStepVisibility() {
    // Erst alle Panes ausblenden
    document.querySelectorAll('.step-pane').forEach(pane => {
        pane.classList.remove('active');
        pane.style.display = 'none';
    });
    
    // Dann den aktiven Schritt einblenden
    const activePane = document.querySelector(`.step-pane[data-step="${currentStep}"]`);
    if (activePane) {
        activePane.classList.add('active');
        activePane.style.display = 'block';
    }
    
    // Update Step Indicators
    document.querySelectorAll('.step-item').forEach(item => {
        const step = parseInt(item.dataset.step);
        item.classList.remove('active', 'completed');
        if (step === currentStep) {
            item.classList.add('active');
        } else if (step < currentStep) {
            item.classList.add('completed');
        }
    });
}

function updateButtonStates() {
    const prevBtn = document.getElementById('btn-prev');
    const nextBtn = document.getElementById('btn-next');
    const saveBtn = document.getElementById('btn-save');
    
    prevBtn.style.display = currentStep > 1 ? 'block' : 'none';
    nextBtn.style.display = currentStep < totalSteps ? 'block' : 'none';
    saveBtn.style.display = currentStep === totalSteps ? 'block' : 'none';
}

function updateProgressBar() {
    const progress = (currentStep - 1) * (100 / (totalSteps - 1));
    document.querySelector('.progress-bar').style.width = `${progress}%`;
}

// Capture Mode Handling
function setupCaptureMode() {
    const captureMode = document.getElementById('capture-mode');
    captureMode.value = 'interval';
    toggleCaptureOptions();
}

function toggleCaptureOptions() {
    const intervalOptions = document.getElementById('interval-options');
    const triggerOptions = document.getElementById('trigger-options');
    
    if (processData.captureMode === 'interval') {
        intervalOptions.style.display = 'block';
        triggerOptions.style.display = 'none';
    } else {
        intervalOptions.style.display = 'none';
        triggerOptions.style.display = 'block';
    }
}

// Header Management
function addHeader() {
    const key = document.getElementById('header-key-p').value.trim();
    const value = document.getElementById('header-value-p').value.trim();
    
    if (!key || !value) {
        alert('Please enter both key and value.');
        return;
    }
    
    processData.headers.push({ name: key, value: value });
    updateHeadersList();
    
    // Felder zurücksetzen
    document.getElementById('header-key-p').value = '';
    document.getElementById('header-value-p').value = '';
}

function updateHeadersList() {
    const headerList = document.getElementById('header-list-p');
    headerList.innerHTML = '';
    
    processData.headers.forEach((header, index) => {
        const li = document.createElement('li');
        li.className = 'list-group-item d-flex justify-content-between align-items-center';
        li.innerHTML = `
            <span><strong>${header.name}</strong>: ${header.value}</span>
            <button class="btn btn-danger btn-sm" onclick="removeHeader(${index})">Remove</button>
        `;
        headerList.appendChild(li);
    });
}

function removeHeader(index) {
    processData.headers.splice(index, 1);
    updateHeadersList();
}

// Wizard zurücksetzen
function resetWizard() {
    currentStep = 1;
    processData = {
        device: '',
        methodParentNode: '',
        methodImageNode: '',
        imageNode: '',
        captureCompletedNode: '',
        readCompletedNode: '',
        captureMode: 'interval',
        interval: '',
        triggerNode: '',
        restUri: '',
        headers: []
    };
    
    updateStepVisibility();
    updateButtonStates();
    updateProgressBar();
    document.querySelectorAll('input').forEach(input => input.value = '');
    document.getElementById('capture-mode').value = 'interval';
    toggleCaptureOptions();
    updateHeadersList();
}

// Speichern des Prozesses
async function saveProcess() {
    try {
        // Prüfe zuerst, ob alle erforderlichen Elemente existieren
        const requiredElements = {
            device: document.getElementById('select-opc-device'),
            methodParentNode: document.getElementById('m-parent-node'),
            methodImageNode: document.getElementById('m-image-node'),
            imageNode: document.getElementById('img-node'),
            capturedNode: document.getElementById('captured-node'),
            completeNode: document.getElementById('read-complete-node'),
            timeout: document.getElementById('timeout-input'),
            captureMode: document.getElementById('capture-mode-select')
        };

        // Prüfe jedes Element
        for (const [key, element] of Object.entries(requiredElements)) {
            if (!element) {
                console.error(`Suche nach Element mit ID: ${key}`);
                throw new Error(`Element '${key}' nicht gefunden`);
            }
        }

        // Sammle die Basisdaten
        const processData = {
            device: requiredElements.device.value,
            methodParentNode: requiredElements.methodParentNode.value,
            methodImageNode: requiredElements.methodImageNode.value,
            imageNode: requiredElements.imageNode.value,
            capturedNode: requiredElements.capturedNode.value,
            completeNode: requiredElements.completeNode.value,
            timeout: parseInt(requiredElements.timeout.value) || 30,
            captureMode: requiredElements.captureMode.value,
            additionalInput: '',
            status: 'active'
        };

        // Hole die zusätzlichen Elemente basierend auf dem captureMode
        if (processData.captureMode === 'interval') {
            const intervalInput = document.getElementById('interval-time-input');
            if (!intervalInput) {
                throw new Error('Interval Input nicht gefunden');
            }
            processData.additionalInput = intervalInput.value;
        } else if (processData.captureMode === 'trigger') {
            const triggerNode = document.getElementById('trigger-node-input');
            if (!triggerNode) {
                throw new Error('Trigger Node Input nicht gefunden');
            }
            processData.additionalInput = triggerNode.value;
        }

        // Debug-Ausgabe
        console.log('ProcessData:', processData);

        // Validiere die Pflichtfelder
        const missingFields = [];
        ['device', 'methodParentNode', 'methodImageNode', 'imageNode', 'capturedNode', 'completeNode'].forEach(field => {
            if (!processData[field]) {
                missingFields.push(field);
            }
        });

        if (missingFields.length > 0) {
            throw new Error(`Please fill in all required fields: ${missingFields.join(', ')}`);
        }

        // Sende die Daten
        const response = await fetch('/api/add-img-process', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(processData)
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Fehler beim Speichern: ${response.status} - ${errorText}`);
        }

        alert('Image capture process successfully created!');
        
        // Modal schließen und Tabelle aktualisieren
        const modal = bootstrap.Modal.getInstance(document.getElementById('modal-new-img-process'));
        if (modal) {
            modal.hide();
            await fetchAndPopulateImgProcesses();
        }

    } catch (error) {
        console.error('Fehler beim Speichern des Prozesses:', error);
        alert('Fehler beim Speichern: ' + error.message);
    }
}

// Aktualisiere die Event-Listener für den Capture Mode
document.getElementById('capture-mode')?.addEventListener('change', function(e) {
    const intervalInputGroup = document.getElementById('interval-input-group');
    const triggerInputGroup = document.getElementById('trigger-input-group');
    
    if (e.target.value === 'interval') {
        intervalInputGroup.style.display = 'block';
        triggerInputGroup.style.display = 'none';
    } else if (e.target.value === 'trigger') {
        intervalInputGroup.style.display = 'none';
        triggerInputGroup.style.display = 'block';
    }
});

// Aktualisiere die Review-Ansicht im letzten Schritt
function updateReviewContent() {
    const reviewContent = document.querySelector('.review-content');
    reviewContent.innerHTML = `
        <div class="mb-3">
            <h5>Device</h5>
            <p>${processData.device}</p>
        </div>
        <div class="mb-3">
            <h5>Method Nodes</h5>
            <p>Parent Node: ${processData.methodParentNode}</p>
            <p>Image Node: ${processData.methodImageNode}</p>
        </div>
        <div class="mb-3">
            <h5>Status Nodes</h5>
            <p>Image Node: ${processData.imageNode}</p>
            <p>Capture Completed: ${processData.captureCompletedNode}</p>
            <p>Read Completed: ${processData.readCompletedNode}</p>
        </div>
        <div class="mb-3">
            <h5>Capture Configuration</h5>
            <p>Mode: ${processData.captureMode}</p>
            ${processData.captureMode === 'interval' 
                ? `<p>Interval: ${processData.interval} seconds</p>`
                : `<p>Trigger Node: ${processData.triggerNode}</p>`}
        </div>
        <div class="mb-3">
            <h5>REST Configuration</h5>
            <p>URI: ${processData.restUri}</p>
            ${processData.headers.length > 0 
                ? `<p>Headers: ${processData.headers.map(h => `${h.name}: ${h.value}`).join(', ')}</p>`
                : '<p>No headers configured</p>'}
        </div>
    `;
}

// Funktion zum Laden und Anzeigen der Geräte im Process-Modal
async function fetchAndPopulateDevicesProcess() {
    const select = document.getElementById('select-opc-device');
    if (!select) {
        throw new Error('OPC Device Select Element nicht gefunden');
    }

    try {
        const response = await fetch('/api/list-opc-ua-devices');
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Fehler beim Laden der OPC UA Geräte: ${response.status} - ${errorText}`);
        }
        
        const data = await response.json();
        
        // Leere das Select
        select.innerHTML = '<option value="" selected disabled>Please select a device...</option>';
        
        // Prüfe ob data.devices ein Array ist
        if (!data || !Array.isArray(data.devices)) {
            console.error('Erhaltene Daten:', data);
            throw new Error('Unerwartetes Datenformat von der API');
        }
        
        // Füge die OPC UA Geräte hinzu
        data.devices.forEach(deviceString => {
            // Prüfe ob der String das erwartete Format hat
            if (typeof deviceString !== 'string') {
                console.warn('Ungültiges Geräteformat:', deviceString);
                return; // Überspringe ungültige Einträge
            }
            
            // Extrahiere ID und Namen aus dem String (Format: "ID - Name")
            const [id, ...nameParts] = deviceString.split(' - ');
            if (!id) {
                console.warn('Keine gültige ID gefunden:', deviceString);
                return;
            }
            
            const option = document.createElement('option');
            option.value = id.trim();
            option.textContent = deviceString;
            select.appendChild(option);
        });
        
        if (select.children.length <= 1) { // Nur die Default-Option
            throw new Error('No valid devices found');
        }
        
    } catch (error) {
        console.error('Fehler beim Laden der OPC UA Geräte:', error);
        throw error;
    }
}

// Aktualisiere die fetchAndPopulateImgProcesses Funktion
async function fetchAndPopulateImgProcesses() {
    try {
        const tableBody = document.querySelector('#table-image-capture-process tbody');
        if (!tableBody) {
            console.warn('Tabelle für Image Capture Processes nicht gefunden');
            return;
        }

        const response = await fetch('/api/list-img-processes');
        if (!response.ok) {
            throw new Error('Fehler beim Laden der Prozesse');
        }
        
        const data = await response.json();
        tableBody.innerHTML = '';
        
        if (!data || !Array.isArray(data)) {
            tableBody.innerHTML = `
                <tr>
                    <td colspan="9" class="text-center">No processes found</td>
                </tr>
            `;
            return;
        }
        
        data.forEach(process => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${process.device || '-'}</td>
                <td>${process.methodParentNode || '-'}</td>
                <td>${process.methodImageNode || '-'}</td>
                <td>${process.imageNode || '-'}</td>
                <td>${process.capturedNode || '-'}</td>
                <td>${process.completeNode || '-'}</td>
                <td>${process.captureMode === 'interval' ? 
                    `Interval: ${process.additionalInput}s` : 
                    `Trigger: ${process.additionalInput}`}</td>
                <td>${process.status || '-'}</td>
                <td>
                    <button class="btn btn-primary btn-sm edit-process" data-process-id="${process.id}">
                        <i class="fas fa-pen"></i>
                    </button>
                    <button class="btn btn-danger btn-sm delete-process" data-process-id="${process.id}">
                        <i class="fas fa-trash"></i>
                    </button>
                </td>
            `;

            // Event-Listener für Edit und Delete
            const editBtn = row.querySelector('.edit-process');
            const deleteBtn = row.querySelector('.delete-process');

            if (editBtn) {
                editBtn.addEventListener('click', () => editProcess(process.id));
            }

            if (deleteBtn) {
                deleteBtn.addEventListener('click', () => {
                    if (confirm('Do you really want to delete this process?')) {
                        deleteProcess(process.id);
                    }
                });
            }

            tableBody.appendChild(row);
        });
        
    } catch (error) {
        console.error('Fehler beim Laden der Prozesse:', error);
        const tableBody = document.querySelector('#table-image-capture-process tbody');
        if (tableBody) {
            tableBody.innerHTML = `
                <tr>
                    <td colspan="9" class="text-center text-danger">
                        <i class="fas fa-exclamation-circle"></i> 
                        Error loading processes: ${error.message}
                    </td>
                </tr>
            `;
        }
    }
}

// Hilfsfunktionen für Edit und Delete
async function editProcess(processId) {
    try {
        const response = await fetch(`/api/img-process/${processId}`);
        if (!response.ok) throw new Error('Fehler beim Laden des Prozesses');
        
        const process = await response.json();
        
        // Öffne das Modal und fülle die Daten
        const modal = new bootstrap.Modal(document.getElementById('modal-new-img-process'));
        
        // Setze die Formulardaten
        document.getElementById('select-opc-device').value = process.device;
        document.getElementById('method-parent-node-input').value = process.methodParentNode;
        document.getElementById('method-image-node-input').value = process.methodImageNode;
        document.getElementById('image-node-input').value = process.imageNode;
        document.getElementById('capture-completed-node-input').value = process.capturedNode;
        document.getElementById('read-completed-node-input').value = process.completeNode;
        document.getElementById('timeout-input').value = process.timeout || 30;
        document.getElementById('capture-mode-select').value = process.captureMode;
        document.getElementById('rest-uri-input').value = process.restUri;
        document.getElementById('header-list-p').innerHTML = process.headers.map(header => `
            <li class="list-group-item d-flex justify-content-between align-items-center">
                <span><strong>${header.name}</strong>: ${header.value}</span>
                <button class="btn btn-danger btn-sm" onclick="removeHeader(${index})">Remove</button>
            </li>
        `).join('');
        
        // Setze die zusätzlichen Inputs basierend auf dem Capture Mode
        if (process.captureMode === 'interval') {
            document.getElementById('interval-input').value = process.additionalInput;
        } else {
            document.getElementById('trigger-node-input').value = process.additionalInput;
        }
        
        // Speichere die Process ID für das Speichern
        document.getElementById('btn-save').dataset.processId = processId;
        
        modal.show();
    } catch (error) {
        console.error('Fehler beim Laden des Prozesses:', error);
        alert('Fehler beim Laden des Prozesses: ' + error.message);
    }
}

async function deleteProcess(processId) {
    try {
        const response = await fetch(`/api/img-process/${processId}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) throw new Error('Fehler beim Löschen des Prozesses');
        
        // Aktualisiere die Tabelle
        await fetchAndPopulateImgProcesses();
        
    } catch (error) {
        console.error('Fehler beim Löschen des Prozesses:', error);
        alert('Fehler beim Löschen des Prozesses: ' + error.message);
    }
}

// Event-Listener für den Save-Button
document.addEventListener('DOMContentLoaded', function() {
    const saveButton = document.getElementById('btn-save');
    if (saveButton) {
        saveButton.addEventListener('click', saveProcess);
    }
}); 