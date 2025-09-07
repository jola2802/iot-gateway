let wsDevices;

// Funktion zum Initialisieren der WebSocket-Verbindung
async function initWebSocket() {
    const response = await fetch ("/api/ws-token", {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
        },
    });

    const token = await response.json();

    wsDevices = new WebSocket(`/api/ws-device-data?token=${token.token}`);

    wsDevices.onopen = () => {
        console.log('WebSocket-Verbindung hergestellt');
    };

    wsDevices.onmessage = (event) => {
        const devices = JSON.parse(event.data);
        updateDeviceTables(devices);
        // updateChart(devices);
    };

    wsDevices.onerror = (error) => {
        console.error('WebSocket-Fehler:', error);
        window.location.reload();
    };

    wsDevices.onclose = (event) => {
        console.warn('WebSocket-Verbindung geschlossen:', event.reason);
        window.location.reload();
    };
}

// Verbesserte REST-API-Funktionen mit Retry-Logik
async function apiRequest(url, options = {}, retries = 3) {
    const defaultOptions = {
        headers: {
            'Content-Type': 'application/json',
        },
        ...options
    };

    for (let attempt = 1; attempt <= retries; attempt++) {
        try {
            const response = await fetch(url, defaultOptions);
            
            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`HTTP ${response.status}: ${errorText}`);
            }
            
            // Prüfe Content-Type der Antwort
            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                try {
                    return await response.json();
                } catch (jsonError) {
                    console.error('JSON Parse Error:', jsonError);
                    // Response-Body kann nicht erneut gelesen werden, daher nur die Fehlermeldung
                    throw new Error(`JSON Parse Error: ${jsonError.message}`);
                }
            } else {
                // Falls keine JSON-Antwort, Text zurückgeben
                const responseText = await response.text();
                console.warn('Non-JSON response received:', responseText);
                return { message: responseText };
            }
        } catch (error) {
            console.error(`API-Anfrage fehlgeschlagen (Versuch ${attempt}/${retries}):`, error);
            
            if (attempt === retries) {
                throw error;
            }
            
            // Warte vor erneutem Versuch (exponentieller Backoff)
            const delay = Math.min(1000 * Math.pow(2, attempt - 1), 5000);
            await new Promise(resolve => setTimeout(resolve, delay));
        }
    }
}

// Verbesserte Geräte-Hinzufügung
document.getElementById('btn-add-new-device').addEventListener('click', async () => {
    const button = document.getElementById('btn-add-new-device');
    const originalText = button.innerHTML;
    
    try {
        // Button-Status ändern
        button.disabled = true;
        button.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Save';
        
        // Erfassung der Eingabewerte
        const deviceData = {
            deviceName: document.getElementById('device-name').value,
            deviceType: document.getElementById('select-device-type').value,
            address: document.getElementById('address')?.value || document.querySelector('#s7-config [placeholder="192.168.2.100:102"]')?.value ||'',
            securityPolicy: document.getElementById('select-security-policy')?.value || '',
            securityMode: document.getElementById('select-security-mode')?.value || '',
            acquisitionTime: parseInt(document.getElementById('acquisition-time-opc-ua')?.value || 
                                      document.getElementById('acquisition-time-s7')?.value || '0', 10),
            username: document.getElementById('username')?.value || '',
            password: document.getElementById('password')?.value || '',
            rack: document.querySelector('#s7-config [placeholder="0"]')?.value || '',
            slot: document.querySelector('#s7-config [placeholder="1"]')?.value || '',
        };

        // Validierung
        const validationErrors = validateDeviceData(deviceData);
        if (validationErrors.length > 0) {
            showNotification('Fehler', validationErrors.join('<br>'), 'error');
            return;
        }

        // API-Request senden
        await apiRequest('/api/add-device', {
            method: 'POST',
            body: JSON.stringify(deviceData),
        });

        showNotification('Erfolg', 'Gerät erfolgreich hinzugefügt!', 'success');
        
        // Modal schließen und Seite aktualisieren
        const modal = bootstrap.Modal.getInstance(document.getElementById('modal-new-device'));
        if (modal) {
            modal.hide();
        }
        
        // Kurze Verzögerung vor Reload
        setTimeout(() => {
            window.location.reload();
        }, 1000);
        
    } catch (error) {
        console.error('Fehler beim Speichern des Geräts:', error);
        showNotification('Fehler', `Fehler beim Hinzufügen des Geräts: ${error.message}`, 'error');
    } finally {
        // Button-Status zurücksetzen
        button.disabled = false;
        button.innerHTML = originalText;
    }
});

// Validierungsfunktion für Gerätedaten
function validateDeviceData(deviceData) {
    const errors = [];
    
    if (!deviceData.deviceName || deviceData.deviceName.trim() === '') {
        errors.push('Gerätename ist erforderlich');
    }
    
    if (deviceData.deviceType === 'opc-ua') {
        if (!deviceData.address || deviceData.address.trim() === '') {
            errors.push('OPC-UA Adresse ist erforderlich');
        }
        if (!deviceData.securityPolicy || deviceData.securityPolicy === '') {
            errors.push('Sicherheitsrichtlinie ist erforderlich');
        }
        if (!deviceData.securityMode || deviceData.securityMode === '') {
            errors.push('Sicherheitsmodus ist erforderlich');
        }
        if (!deviceData.acquisitionTime || deviceData.acquisitionTime < 100) {
            errors.push('Abtastzeit muss mindestens 100ms betragen');
        }
    } else if (deviceData.deviceType === 's7') {
        if (!deviceData.address || deviceData.address.trim() === '') {
            errors.push('S7 Adresse ist erforderlich');
        }
        if (!deviceData.rack || deviceData.rack === '') {
            errors.push('Rack-Nummer ist erforderlich');
        }
        if (!deviceData.slot || deviceData.slot === '') {
            errors.push('Slot-Nummer ist erforderlich');
        }
    }
    
    return errors;
}

// Event-Listener für Save-Button wird dynamisch hinzugefügt
function attachSaveButtonListener() {
    const saveButton = document.getElementById('btn-edit-device');
    if (saveButton) {
        // Entferne alte Event-Listener
        const newButton = saveButton.cloneNode(true);
        saveButton.parentNode.replaceChild(newButton, saveButton);
        
        // Füge neuen Event-Listener hinzu
        newButton.addEventListener('click', async (e) => {
            e.preventDefault();
            e.stopPropagation();
            
            try {
                await saveEditDevice();
            } catch (error) {
                console.error('Fehler beim Speichern:', error);
            }
        });
    }
}

// Verbesserte Geräte-Aktualisierung
async function saveEditDevice() {
    return new Promise(async (resolve, reject) => {
        const button = document.getElementById('btn-edit-device');
        const originalText = button.innerHTML;
        
        try {
            // Button-Status ändern
            button.disabled = true;
            button.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Save';
            
            // Erfassung der Eingabewerte
            const deviceData = {
                deviceName: document.getElementById('device-name-1').value,
                deviceType: document.getElementById('select-device-type-1').value,
                address: document.getElementById('address-1')?.value || document.getElementById('address-2')?.value || '',
                securityPolicy: document.getElementById('select-security-policy-1')?.value || '',
                securityMode: document.getElementById('select-security-mode-1')?.value || '',
                acquisitionTime: parseInt(
                    document.getElementById('acquisition-time-opc-ua-1')?.value ||
                    document.getElementById('acquisition-time-2')?.value || '0',
                    10
                ),
                username: document.getElementById('username')?.value || document.getElementById('username-1')?.value || '',
                password: document.getElementById('password')?.value || document.getElementById('password-1')?.value || '',
                rack: document.querySelector('#rack')?.value || document.querySelector('#s7-config-1 [placeholder="0"]')?.value || '',
                slot: document.querySelector('#slot')?.value || document.querySelector('#s7-config-1 [placeholder="1"]')?.value || '',
                datapoints: Array.from(document.querySelectorAll('#ipi-table tbody tr')).map(row => {
                    const cells = row.querySelectorAll('td');
                    const nameInput = cells[1]?.querySelector('input');
                    const datatypeInput = cells[2]?.querySelector('select');
                    const addressInput = cells[3]?.querySelector('input');

                    const isOpcUa = document.getElementById('select-device-type-1').value === 'opc-ua';
                    return {
                        datapointId: row.querySelector('td').textContent.trim(),
                        name: nameInput ? nameInput.value.trim() : cells[1]?.textContent.trim() || '',
                        datatype: isOpcUa ? '' : datatypeInput ? datatypeInput.value.trim() : cells[2]?.textContent.trim() || '',
                        address: addressInput ? addressInput.value.trim() : cells[3]?.textContent.trim() || '',
                    };
                }).filter(dp => {
                    const isOpcUa = document.getElementById('select-device-type-1').value === 'opc-ua';
                    const hasValidName = dp.name && dp.name.trim() !== '' && dp.name !== 'Enter Name';
                    const hasValidAddress = dp.address && dp.address.trim() !== '' && dp.address !== 'Enter Address / Node ID';
                    const hasValidDatatype = isOpcUa || (dp.datatype && dp.datatype.trim() !== '' && dp.datatype !== '-');
                    
                    if (!hasValidName || !hasValidAddress || !hasValidDatatype) {
                        console.log('Filtere ungültigen Datenpunkt heraus:', dp);
                        return false;
                    }
                    return true;
                }),
            };

            deviceData.deviceId = localStorage.getItem('device_id');

            console.log('Zu aktualisierende Gerätedaten:', deviceData);

            // API-Request senden
            await apiRequest(`/api/update-device/${deviceData.deviceId}`, {
                method: 'POST',
                body: JSON.stringify(deviceData),
            });
            
            // Erfolgreiche Rückmeldung anzeigen
            const validDatapointsCount = deviceData.datapoints.length;
            if (validDatapointsCount > 0) {
                showNotification('Erfolg', `Gerät "${deviceData.deviceName}" erfolgreich aktualisiert mit ${validDatapointsCount} Datenpunkten`, 'success');
            } else {
                showNotification('Erfolg', `Gerät "${deviceData.deviceName}" erfolgreich aktualisiert`, 'success');
            }

            // Warte kurz bevor Modal geschlossen wird
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // Modal ordnungsgemäß schließen
            const modalEl = document.getElementById('modal-edit-device');
            const modalInstance = bootstrap.Modal.getInstance(modalEl);
            if (modalInstance) {
                modalInstance.hide();
                
                // Warte bis das Modal vollständig geschlossen ist
                modalEl.addEventListener('hidden.bs.modal', function onHidden() {
                    // Entferne den Event-Listener
                    modalEl.removeEventListener('hidden.bs.modal', onHidden);
                    
                    // Entferne das graue Overlay manuell falls es noch da ist
                    setTimeout(() => {
                        // Verwende die globale removeModalOverlay Funktion falls verfügbar
                        if (typeof removeModalOverlay === 'function') {
                            removeModalOverlay();
                        } else {
                            // Fallback: Manuelles Entfernen
                            const backdrop = document.querySelector('.modal-backdrop');
                            if (backdrop) {
                                backdrop.remove();
                            }
                            document.body.classList.remove('modal-open');
                            document.body.style.overflow = '';
                            document.body.style.paddingRight = '';
                        }
                        
                        // Aktualisiere die Geräteliste
                        if (typeof fetchAndPopulateDevices === 'function') {
                            fetchAndPopulateDevices();
                        }
                    }, 1000);
                }, { once: true });
            }
            
            resolve();
            
        } catch (error) {
            console.error('❌ Fehler beim Aktualisieren des Geräts:', error);
            showNotification('Fehler', `Fehler beim Speichern des Geräts: ${error.message}`, 'error');
            reject(error);
        } finally {
            // Button-Status zurücksetzen
            button.disabled = false;
            button.innerHTML = originalText;
        }
    });
}

// Benachrichtigungssystem - Toast-Style wie bei Image Capture
function showNotification(title, message, type = 'info') {
    // Toast-Container erstellen, falls er nicht existiert
    let toastContainer = document.getElementById('toast-container');
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.id = 'toast-container';
        toastContainer.className = 'position-fixed top-0 end-0 p-3';
        toastContainer.style.zIndex = '1055';
        document.body.appendChild(toastContainer);
    }
    
    // Icon basierend auf Alert-Typ bestimmen
    let icon = '';
    let bgColor = '';
    const alertType = type === 'error' ? 'danger' : type;
    
    switch(alertType) {
        case 'success':
            icon = '<i class="fas fa-check-circle me-2"></i>';
            bgColor = 'bg-success';
            break;
        case 'danger':
        case 'error':
            icon = '<i class="fas fa-exclamation-triangle me-2"></i>';
            bgColor = 'bg-danger';
            break;
        case 'warning':
            icon = '<i class="fas fa-exclamation-circle me-2"></i>';
            bgColor = 'bg-warning';
            break;
        case 'info':
            icon = '<i class="fas fa-info-circle me-2"></i>';
            bgColor = 'bg-info';
            break;
        default:
            icon = '<i class="fas fa-bell me-2"></i>';
            bgColor = 'bg-primary';
    }
    
    // Neuen Toast erstellen
    const toastId = 'toast-' + Date.now();
    const toastDiv = document.createElement('div');
    toastDiv.id = toastId;
    toastDiv.className = 'toast show';
    toastDiv.setAttribute('role', 'alert');
    toastDiv.innerHTML = `
        <div class="toast-header ${bgColor} text-white">
            ${icon}
            <strong class="me-auto">${title}</strong>
            <button type="button" class="btn-close btn-close-white" data-bs-dismiss="toast"></button>
        </div>
        <div class="toast-body">
            ${message}
        </div>
    `;
    
    // Toast zum Container hinzufügen
    toastContainer.appendChild(toastDiv);
    
    // Auto-dismiss Zeit basierend auf Alert-Typ
    let dismissTime = 5000; // Standard: 5 Sekunden
    if (alertType === 'info') {
        dismissTime = 4000; // Info-Toasts nur 4 Sekunden
    } else if (alertType === 'success') {
        dismissTime = 6000; // Erfolgs-Toasts 6 Sekunden
    } else if (alertType === 'danger' || alertType === 'error') {
        dismissTime = 8000; // Fehler-Toasts 8 Sekunden
    }
    
    // Toast automatisch ausblenden
    setTimeout(() => {
        if (toastDiv.parentNode) {
            toastDiv.classList.remove('show');
            setTimeout(() => {
                if (toastDiv.parentNode) {
                    toastDiv.remove();
                }
            }, 300); // Fade-out Animation abwarten
        }
    }, dismissTime);
    
    // Bootstrap Toast initialisieren (falls verfügbar)
    if (typeof bootstrap !== 'undefined' && bootstrap.Toast) {
        const toast = new bootstrap.Toast(toastDiv, {
            autohide: true,
            delay: dismissTime
        });
        toast.show();
    }
}