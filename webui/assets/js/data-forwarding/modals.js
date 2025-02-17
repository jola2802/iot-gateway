const { elements, state } = window.dataForwarding;

const newImgProcessModal = document.getElementById('modal-new-img-process');
const routeModal = document.getElementById('modal-new-route');
const destinationTypeSelect = document.getElementById('select-destination-type');
const restConfig = document.getElementById('rest-config');
const fileConfig = document.getElementById('file-config');
const mqttConfig = document.getElementById('mqtt-config');
const lupeButtons = document.querySelectorAll('.btn-open-browsed-nodes');

// Event Listeners
if (newImgProcessModal) {
    newImgProcessModal.addEventListener('show.bs.modal', fetchAndPopulateDevicesProcess);
}

// Funktion zum Anzeigen/Verstecken des Spinners
function toggleLoadingSpinner(show) {
    const modalContent = routeModal.querySelector('.modal-content');
    const spinner = routeModal.querySelector('.loading-spinner');
    
    if (!modalContent || !spinner) return;
    
    modalContent.style.opacity = show ? '0.10' : '1';
    spinner.style.display = show ? 'block' : 'none';
}

// Funktion zum Verstecken aller Konfigurationen
function hideAllConfigsRoute() {
    if (restConfig) restConfig.style.display = 'none';
    if (fileConfig) fileConfig.style.display = 'none';
    if (mqttConfig) mqttConfig.style.display = 'none';
}

// Funktion zum Anzeigen der gewählten Konfiguration
function showConfig(type) {
    hideAllConfigsRoute();
    const configMap = {
        'rest': 'rest-config',
        'file-based': 'file-config',
        'mqtt': 'mqtt-config'
    };
    
    const configId = configMap[type];
    if (configId) {
        const config = document.getElementById(configId);
        if (config) {
            config.style.display = 'block';
        }
    }
}

// Überarbeitete updateDeviceList Funktion
async function updateDeviceList() {
    const deviceList = document.getElementById('check-list-devices');
    if (!deviceList) {
        throw new Error('Geräteliste nicht gefunden');
    }
    
    try {
        const devices = await DeviceCache.getDevices();
        if (!Array.isArray(devices) || devices.length === 0) {
            deviceList.innerHTML = '<li class="list-group-item">Keine Geräte verfügbar</li>';
            return;
        }
        
        deviceList.innerHTML = '';
        devices.forEach(device => {
            const li = document.createElement('li');
            li.className = 'list-group-item';
            li.innerHTML = `
                <div class="form-check">
                    <input type="checkbox" class="form-check-input" id="formCheck-${device}" value="${device}">
                    <label class="form-check-label" for="formCheck-${device}">${device}</label>
                </div>
            `;
            deviceList.appendChild(li);
        });
    } catch (error) {
        console.error('Fehler beim Laden der Geräte:', error);
        deviceList.innerHTML = '<li class="list-group-item text-danger">Fehler beim Laden der Geräte</li>';
        throw error;
    }
}

// Initialisierung der Modals
function initializeModals() {
    const newRouteModal = document.getElementById('modal-new-route');
    const newProcessModal = document.getElementById('modal-new-img-process');
    
    if (newRouteModal) {
        new bootstrap.Modal(newRouteModal);
        setupRouteModal(newRouteModal);
    }
    
    if (newProcessModal) {
        new bootstrap.Modal(newProcessModal);
        setupProcessModal(newProcessModal);
    }
}

// Setup für das Route Modal
function setupRouteModal(modal) {
    modal.addEventListener('show.bs.modal', async function(event) {
        toggleLoadingSpinner(true);
        
        try {
            // Initialisiere die Select-Optionen
            initializeSelectOptions();
            
            // Aktualisiere die Geräteliste
            await updateDeviceList();
            
            const button = event.relatedTarget;
            if (button && button.dataset.routeId) {
                await loadRouteData(button.dataset.routeId);
            } else {
                resetRouteForm();
            }
            
        } catch (error) {
            console.error('Fehler beim Laden des Modals:', error);
            alert('Fehler beim Laden des Modals: ' + error.message);
        } finally {
            toggleLoadingSpinner(false);
        }
    });
}

// Setup für das Process Modal
function setupProcessModal(modal) {
    modal.addEventListener('show.bs.modal', async function(event) {
        try {
            await fetchAndPopulateDevicesProcess();
            resetProcessForm();
        } catch (error) {
            console.error('Error initializing process modal:', error);
            alert('Error loading devices: ' + error.message);
        }
    });
}

// Hilfsfunktion zum Zurücksetzen des Prozess-Formulars
function resetProcessForm() {
    const modal = document.getElementById('modal-new-img-process');
    if (!modal) return;
    
    // Setze alle Eingabefelder zurück
    modal.querySelectorAll('input').forEach(input => {
        input.value = '';
    });
    
    // Setze Select-Felder zurück
    modal.querySelectorAll('select').forEach(select => {
        select.selectedIndex = 0;
    });
    
    // Setze die Schrittanzeige zurück
    currentStep = 1;
    updateStepVisibility();
    updateButtonStates();
    updateProgressBar();
}

// Exportiere die benötigten Funktionen
window.dataForwarding.functions = {
    ...window.dataForwarding.functions,
    setRouteData: function(routeData) {
        // Implementiere die Logik zum Setzen der Routendaten im Modal
        const modal = document.getElementById('modal-new-route');
        if (!modal) return;

        // Setze den Titel
        modal.querySelector('.modal-title').textContent = 'Edit Route';
        
        // Setze die Formularwerte
        const destinationTypeSelect = modal.querySelector('#select-destination-type');
        if (destinationTypeSelect) {
            destinationTypeSelect.value = routeData.destinationType;
            // Trigger change event to show/hide correct config
            destinationTypeSelect.dispatchEvent(new Event('change'));
        }

        // Setze REST spezifische Felder
        if (routeData.destinationType === 'rest') {
            const restUri = modal.querySelector('#rest-uri');
            if (restUri) restUri.value = routeData.destination_url || '';
            
            const restInterval = modal.querySelector('#rest-sending-interval');
            if (restInterval) restInterval.value = routeData.interval || '';
        }
        
        // Setze File-based spezifische Felder
        if (routeData.destinationType === 'file-based') {
            const fbFilename = modal.querySelector('#fb-input-filename');
            if (fbFilename) fbFilename.value = routeData.filePath || '';
            
            const fbInterval = modal.querySelector('#fb-sending-interval');
            if (fbInterval) fbInterval.value = routeData.interval || '';
        }

        // Header setzen
        if (routeData.headers && Array.isArray(routeData.headers)) {
            const headerList = document.getElementById('header-list-r');
            headerList.innerHTML = '';
            
            routeData.headers.forEach(header => {
                const headerKeyInput = { value: header.name };
                const headerValueInput = { value: header.value };
                addHeaderToList(headerKeyInput, headerValueInput, headerList);
            });
        }
    }
};

// Initialisierung beim Laden der Seite
document.addEventListener('DOMContentLoaded', initializeModals);

// Funktion zum Zurücksetzen des Formulars
function resetRouteForm() {
    routeModal.querySelector('.modal-title').textContent = 'Add new Route';
    document.getElementById('btn-save-route').textContent = 'Save';
    routeModal.querySelector('form').reset();
    document.getElementById('header-list-r').innerHTML = '';
    hideAllConfigsRoute();
    destinationTypeSelect.value = 'rest';
    restConfig.style.display = 'block';
}

// Initialisiere Select-Optionen
function initializeSelectOptions() {
    // Destination Types
    destinationTypeSelect.innerHTML = `
        <optgroup label="Destination Type">
            <option value="rest">REST</option>
            <option value="file-based">File-based</option>
            <option value="mqtt">MQTT</option>
        </optgroup>
    `;

    // Data Format
    const dataFormatSelect = document.getElementById('select-data-format');
    dataFormatSelect.innerHTML = `
        <optgroup label="Data Format">
            <option value="json">JSON</option>
            <option value="xml">XML</option>
        </optgroup>
    `;
}

// Event-Listener für den Save-Button
document.getElementById('btn-save-route').addEventListener('click', async function() {
    try {
        toggleLoadingSpinner(true);
        await saveRoute();
        const modal = bootstrap.Modal.getInstance(routeModal);
        modal.hide();
        await fetchAndPopulateDataForwarding();
    } catch (error) {
        console.error('Fehler beim Speichern:', error);
        alert('Fehler beim Speichern: ' + error.message);
    } finally {
        toggleLoadingSpinner(false);
    }
});

// Überarbeitete loadRouteData Funktion
async function loadRouteData(routeId) {
    try {
        toggleLoadingSpinner(true);

        const response = await fetch(`/api/route/${routeId}`);
        if (!response.ok) {
            throw new Error(`Fehler beim Laden der Route-Daten: ${response.status}`);
        }
        
        const routeData = await response.json();

        // Modal-Titel ändern
        routeModal.querySelector('.modal-title').textContent = 'Edit Route';
        
        // Save-Button aktualisieren
        const saveButton = document.getElementById('btn-save-route');
        saveButton.dataset.routeId = routeId;
        saveButton.textContent = 'Update';

        // Destination Type setzen
        destinationTypeSelect.value = routeData.destinationType;
        
        // Konfiguration anzeigen
        hideAllConfigsRoute();
        switch (routeData.destinationType) {
            case 'rest':
                restConfig.style.display = 'block';
                document.getElementById('rest-uri').value = routeData.destination_url || '';
                document.getElementById('rest-sending-interval').value = routeData.interval || '';
                break;
            case 'file-based':
                fileConfig.style.display = 'block';
                document.getElementById('fb-input-filename').value = routeData.filePath || '';
                document.getElementById('fb-sending-interval').value = routeData.interval || '';
                break;
        }

        // Warte auf die Geräte-Checkboxen
        await waitForDeviceCheckboxes();
        
        // Setze die ausgewählten Geräte
        const selectedDevices = Array.isArray(routeData.devices) 
            ? routeData.devices.map(device => typeof device === 'string' ? device.replace(/[{"}]/g, '') : device)
            : [];
            
        document.querySelectorAll('#check-list-devices input[type="checkbox"]').forEach(checkbox => {
            checkbox.checked = selectedDevices.includes(checkbox.value);
        });

        // Header setzen
        if (routeData.destinationType === 'rest' && routeData.headers) {
            const headerList = document.getElementById('header-list-r');
            headerList.innerHTML = '';
            
            const headers = Array.isArray(routeData.headers) ? routeData.headers : [];
            headers.forEach(header => {
                if (header && header.name && header.value) {
                    const listItem = document.createElement('li');
                    listItem.classList.add('list-group-item', 'd-flex', 'justify-content-between', 'align-items-center');
                    listItem.dataset.headerKey = header.name;
                    listItem.dataset.headerValue = header.value;
                    
                    listItem.innerHTML = `
                        <span><strong>${header.name}</strong>: ${header.value}</span>
                        <button class="btn btn-danger btn-sm remove-header-button" type="button">Remove</button>
                    `;
                    
                    const deleteButton = listItem.querySelector('.remove-header-button');
                    deleteButton.addEventListener('click', () => {
                        headerList.removeChild(listItem);
                    });
                    
                    headerList.appendChild(listItem);
                }
            });
        }

    } catch (error) {
        console.error('Fehler beim Laden der Route-Daten:', error);
        alert('Fehler beim Laden der Route-Daten: ' + error.message);
    } finally {
        toggleLoadingSpinner(false);
    }
}

// Hilfsfunktion zum Warten auf Geräte-Checkboxen
function waitForDeviceCheckboxes() {
    return new Promise((resolve) => {
        const checkInterval = setInterval(() => {
            const checkboxes = document.querySelectorAll('#check-list-devices input[type="checkbox"]');
            if (checkboxes.length > 0) {
                clearInterval(checkInterval);
                resolve();
            }
        }, 100);

        setTimeout(() => {
            clearInterval(checkInterval);
            resolve();
        }, 5000);
    });
}

// Funktion zum Speichern einer Route
async function saveRoute() {
    try {
        toggleLoadingSpinner(true);
        
        // Sammle die Header
        const headers = [];
        document.querySelectorAll('#header-list-r li').forEach(li => {
            headers.push({
                name: li.dataset.headerKey,
                value: li.dataset.headerValue
            });
        });
        
        // Sammle die restlichen Daten
        const routeData = {
            destinationType: document.getElementById('select-destination-type').value,
            dataFormat: document.getElementById('select-data-format').value,
            headers: headers,
            devices: [],
            destinationurl: '',
            interval: 0,
            filePath: '',
            destination_url: '',
            interval: 0
        };
        
        // Füge die ausgewählten Geräte hinzu
        document.querySelectorAll('#check-list-devices input[type="checkbox"]:checked').forEach(checkbox => {
            routeData.devices.push(checkbox.value);
        });
        
        // Füge spezifische Konfiguration basierend auf dem Typ hinzu
        if (routeData.destinationType === 'rest') {
            routeData.destinationurl = document.getElementById('rest-uri').value;
            routeData.interval = parseInt(document.getElementById('rest-sending-interval').value, 10) || 0;
        } else if (routeData.destinationType === 'file-based') {
            routeData.filePath = document.getElementById('fb-input-filename').value;
            routeData.interval = parseInt(document.getElementById('fb-sending-interval').value, 10) || 0;
        }
        
        // Bestimme die HTTP-Methode und URL
        const routeId = document.getElementById('btn-save-route').dataset.routeId;
        const url = routeId ? `/api/route/${routeId}` : '/api/add-route';
        const method = routeId ? 'PUT' : 'POST';
        
        const response = await fetch(url, {
            method: method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(routeData)
        });
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        alert(routeId ? 'Route erfolgreich aktualisiert!' : 'Neue Route erfolgreich erstellt!');
        
        const modalInstance = bootstrap.Modal.getInstance(routeModal);
        modalInstance.hide();
        
        await fetchAndPopulateDataForwarding();
        
    } catch (error) {
        console.error('Fehler beim Speichern der Route:', error);
        alert('Fehler beim Speichern der Route: ' + error.message);
    } finally {
        toggleLoadingSpinner(false);
    }
}

// Header-Funktionalität für Routes
function initializeHeaderFunctionality() {
    // Add Header Button für Routes
    const addHeaderButtonRoute = document.getElementById('add-header-button-r');
    if (addHeaderButtonRoute) {
        addHeaderButtonRoute.addEventListener('click', function() {
            const headerKeyInput = document.getElementById('header-key-r');
            const headerValueInput = document.getElementById('header-value-r');
            const headerList = document.getElementById('header-list-r');
            
            addHeaderToList(headerKeyInput, headerValueInput, headerList);
        });
    }
}

// Funktion zum Hinzufügen von Headern
function addHeaderToList(keyInput, valueInput, list) {
    if (!keyInput || !valueInput || !list) {
        console.error('Erforderliche Elemente für Header-Hinzufügung fehlen');
        return;
    }

    const key = keyInput.value?.trim();
    const value = valueInput.value?.trim();
    
    if (!key || !value) {
        alert('Bitte geben Sie sowohl Key als auch Value ein.');
        return;
    }
    
    const listItem = document.createElement('li');
    listItem.className = 'list-group-item d-flex justify-content-between align-items-center';
    listItem.dataset.headerKey = key;
    listItem.dataset.headerValue = value;
    
    listItem.innerHTML = `
        <span><strong>${key}</strong>: ${value}</span>
        <button class="btn btn-danger btn-sm remove-header-button" type="button">Remove</button>
    `;
    
    const deleteButton = listItem.querySelector('.remove-header-button');
    deleteButton.addEventListener('click', () => {
        list.removeChild(listItem);
    });
    
    list.appendChild(listItem);
    
    // Eingabefelder leeren
    keyInput.value = '';
    valueInput.value = '';
}

// Füge die Initialisierung zum DOMContentLoaded Event hinzu
document.addEventListener('DOMContentLoaded', () => {
    initializeHeaderFunctionality();
}); 