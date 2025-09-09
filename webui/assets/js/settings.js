document.addEventListener('DOMContentLoaded', function() {
    
    // Initialisiere die Filter für die Log-Levels
    initializeLogFilters();
    
    // Settings beim Laden der Seite abrufen
    loadSettings();
    
    // Logs beim Laden der Seite abrufen
    fetchLogs();

    // Automatische Aktualisierung der Logs alle 30 Sekunden
    setInterval(fetchLogs, 30000);
});

/**
 * Initialisiere die Filter für die Log-Levels
 */
function initializeLogFilters() {
    const filterContainer = document.getElementById('logLevelFilters');
    if (!filterContainer) return;
    
    // Log-Level-Filter erstellen
    const logLevels = [
        { id: 'level-info', label: 'INFO', className: 'text-info', checked: true },
        { id: 'level-warn', label: 'WARN', className: 'text-warning', checked: true },
        { id: 'level-error', label: 'ERROR', className: 'text-danger', checked: true },
        { id: 'level-debug', label: 'DEBUG', className: 'text-secondary', checked: true }
    ];
    
    // Filter-UI erstellen
    logLevels.forEach(level => {
        const formCheck = document.createElement('div');
        formCheck.className = 'form-check form-check-inline';
        
        const input = document.createElement('input');
        input.className = 'form-check-input';
        input.type = 'checkbox';
        input.id = level.id;
        input.checked = level.checked;
        input.addEventListener('change', applyLogFilters);
        
        const label = document.createElement('label');
        label.className = `form-check-label ${level.className} fw-bold`;
        label.htmlFor = level.id;
        label.textContent = level.label;
        
        formCheck.appendChild(input);
        formCheck.appendChild(label);
        filterContainer.appendChild(formCheck);
    });
    
    // Erstelle einen "Clear Logs" Button
    const clearButton = document.createElement('button');
    clearButton.className = 'btn btn-sm btn-secondary ms-3';
    clearButton.textContent = 'Logs löschen';
    clearButton.addEventListener('click', clearLogs);
    filterContainer.appendChild(clearButton);
}

/**
 * Filtere die Logs basierend auf den ausgewählten Log-Levels
 */
function applyLogFilters() {
    const logEntries = document.querySelectorAll('.log-entry');
    
    // Prüfe, welche Filter aktiviert sind
    const showInfo = document.getElementById('level-info')?.checked || false;
    const showWarn = document.getElementById('level-warn')?.checked || false;
    const showError = document.getElementById('level-error')?.checked || false;
    const showDebug = document.getElementById('level-debug')?.checked || false;
    
    // Wende die Filter an
    logEntries.forEach(entry => {
        const level = entry.getAttribute('data-level');
        
        let isVisible = false;
        switch (level) {
            case 'INFO':
                isVisible = showInfo;
                break;
            case 'WARN':
                isVisible = showWarn;
                break;
            case 'ERROR':
                isVisible = showError;
                break;
            case 'DEBUG':
                isVisible = showDebug;
                break;
            default:
                isVisible = true;
        }
        
        entry.style.display = isVisible ? '' : 'none';
    });
}

/**
 * Löscht alle Logs (sendet Anfrage zum Löschen an den Server)
 */
function clearLogs() {
    if (confirm('Möchten Sie wirklich alle Logs löschen?')) {
        fetch('/api/logs/clear', {
            method: 'POST',
            headers: {
                'Cache-Control': 'no-cache',
                'Content-Type': 'application/json'
            }
        })
        .then(response => {
            if (!response.ok) {
                throw new Error('Konnte Logs nicht löschen');
            }
            return response.json();
        })
        .then(() => {
            // Logs nach dem Löschen neu laden
            fetchLogs();
        })
        .catch(error => {
            console.error('Fehler beim Löschen der Logs:', error);
            alert('Fehler beim Löschen der Logs: ' + error.message);
        });
    }
}

/**
 * Abrufen der Logs vom Server und Anzeigen im UI
 */
function fetchLogs() {
    // Cache verhindern durch zufälligen Query-Parameter
    const timestamp = new Date().getTime();
    fetch(`/api/logs?_=${timestamp}`, {
        headers: {
            'Cache-Control': 'no-cache, no-store, must-revalidate',
            'Pragma': 'no-cache',
            'Expires': '0'
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        // console.log("Log data received:", data.info); // Debug-Info

        const logsContent = document.getElementById('logsContent');
        if (!logsContent) return;
            
        if (data.logs && data.logs.length > 0) {
            // Parsen und Formatieren der Logs
            const logEntries = parseAndFormatLogs(data.logs);
            
            // Anzeigen der Logs in der Tabelle
            displayLogsAsTable(logEntries, logsContent);
            
            // Filter anwenden
            applyLogFilters();
            
            // Automatisch nach unten scrollen
            logsContent.scrollTop = logsContent.scrollHeight;
        } else {
            logsContent.innerHTML = '<div class="alert alert-warning">No Logs available</div>';
        }
    })
    .catch(error => {
        console.error('Fehler beim Abrufen der Logs:', error);
        const logsContent = document.getElementById('logsContent');
        if (logsContent) {
            logsContent.innerHTML = `<div class="alert alert-danger">Error loading Logs: ${error.message}</div>`;
        }
    });
}

/**
 * Parst und formatiert die Logs
 * @param {string} logsString - Der Logs-String vom Server
 * @return {Array} Ein Array von formatierten Log-Einträgen
 */
function parseAndFormatLogs(logsString) {
    const logEntries = [];
    
    // Aufteilen in Zeilen
    const logLines = logsString.split('\n');
    
    logLines.forEach(line => {
        if (!line.trim()) return; // Leere Zeilen überspringen
        
        try {
            const logEntry = JSON.parse(line);
            
            // Nur verarbeiten, wenn es ein gültiges JSON-Objekt ist
            if (logEntry && logEntry.time && logEntry.level && logEntry.msg) {
                const timestamp = new Date(logEntry.time);
                const formattedDate = timestamp.toLocaleDateString();
                const formattedTime = timestamp.toLocaleTimeString();
                const level = logEntry.level.toUpperCase();
                const message = logEntry.msg;
                
                // Extrahiere zusätzliche Felder (außer time, level, msg)
                const fields = {};
                Object.keys(logEntry).forEach(key => {
                    if (!['time', 'level', 'msg'].includes(key)) {
                        fields[key] = logEntry[key];
                    }
                });
                
                logEntries.push({
                    date: formattedDate,
                    time: formattedTime,
                    level: level,
                    message: message,
                    fields: fields,
                    rawEntry: logEntry
                });
            }
        } catch (e) {
            // Wenn das Parsen fehlschlägt, füge die Rohzeile hinzu
            if (line.trim()) {
                logEntries.push({
                    date: '-',
                    time: '-',
                    level: 'UNKNOWN',
                    message: line,
                    fields: {},
                    isRaw: true
                });
            }
        }
    });
    
    return logEntries;
}

/**
 * Zeigt die Logs in einer Tabelle an
 * @param {Array} logEntries - Die formatierten Log-Einträge
 * @param {HTMLElement} container - Das Container-Element für die Logs
 */
function displayLogsAsTable(logEntries, container) {
    if (logEntries.length === 0) {
        container.innerHTML = '<div class="alert alert-warning">No valid Log entries found</div>';
        return;
    }
    
    // Beginne mit einer neuen Tabelle
    let html = `
        <div class="table-responsive">
            <table class="table table-sm table-hover log-table">
                <thead>
                    <tr>
                        <th style="width: 10%">Date</th>
                        <th style="width: 10%">Time</th>
                        <th style="width: 10%">Level</th>
                        <th style="width: 50%">Message</th>
                        <th style="width: 20%">Details</th>
                    </tr>
                </thead>
                <tbody>
    `;
    
    // Füge Zeilen für jedes Log-Entry hinzu
    logEntries.forEach(entry => {
        const levelClass = getLevelClass(entry.level);
        
        // Bereite Details vor
        let details = '';
        if (!entry.isRaw && Object.keys(entry.fields).length > 0) {
            details = Object.entries(entry.fields)
                .map(([key, value]) => `<span class="badge bg-secondary">${key}: ${JSON.stringify(value)}</span>`)
                .join(' ');
        }
        
        html += `
            <tr class="log-entry" data-level="${entry.level}">
                <td>${entry.date}</td>
                <td>${entry.time}</td>
                <td><span class="badge ${levelClass}">${entry.level}</span></td>
                <td class="log-message">${escapeHtml(entry.message)}</td>
                <td class="log-details">${details}</td>
            </tr>
        `;
    });
    
    html += `
                </tbody>
            </table>
        </div>
    `;
    
    container.innerHTML = html;
}

/**
 * Ermittelt die CSS-Klasse für ein Log-Level
 * @param {string} level - Das Log-Level
 * @return {string} Die CSS-Klasse
 */
function getLevelClass(level) {
    switch (level) {
        case 'ERROR':
            return 'bg-danger';
        case 'WARN':
            return 'bg-warning';
        case 'INFO':
            return 'bg-info';
        case 'DEBUG':
            return 'bg-secondary';
        default:
            return 'bg-light text-dark';
    }
}

/**
 * Hilfsfunktion zum Escapen von HTML
 * @param {string} html - Der zu escapende String
 * @return {string} Der escapte String
 */
function escapeHtml(html) {
    if (typeof html !== 'string') {
        return String(html);
    }
    
    const escapeChars = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
    };
    
    return html.replace(/[&<>"']/g, function(char) {
        return escapeChars[char];
    });
}

// ==================== SETTINGS MANAGEMENT ====================

/**
 * Lädt alle Systemeinstellungen vom Server
 */
function loadSettings() {
    fetch('/api/settings')
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            displaySettings(data.settings);
        })
        .catch(error => {
            console.error('Fehler beim Laden der Einstellungen:', error);
            const settingsContent = document.getElementById('settingsContent');
            if (settingsContent) {
                settingsContent.innerHTML = `<div class="alert alert-danger">Fehler beim Laden der Einstellungen: ${error.message}</div>`;
            }
        });
}

/**
 * Zeigt die Einstellungen in kategorisierten Bereichen an
 * @param {Array} settings - Array von Einstellungsobjekten
 */
function displaySettings(settings) {
    const settingsContent = document.getElementById('settingsContent');
    if (!settingsContent) return;

    // Gruppiere Einstellungen nach Kategorien
    const categories = {};
    settings.forEach(setting => {
        if (!categories[setting.category]) {
            categories[setting.category] = [];
        }
        categories[setting.category].push(setting);
    });

    let html = '';
    
    // Definiere Kategorienamen und Icons
    const categoryInfo = {
        'integration': { name: 'Integration Services', icon: 'fas fa-plug' },
        'mqtt': { name: 'MQTT Broker', icon: 'fas fa-broadcast-tower' },
        'system': { name: 'System', icon: 'fas fa-cog' },
        'backup': { name: 'Backup & Retention', icon: 'fas fa-database' },
        'docker': { name: 'Docker Configuration', icon: 'fab fa-docker' }
    };

    // Erstelle HTML für jede Kategorie
    Object.keys(categories).forEach(category => {
        const categorySettings = categories[category];
        const info = categoryInfo[category] || { name: category, icon: 'fas fa-cog' };
        
        html += `
            <div class="settings-section">
                <h5><i class="${info.icon}"></i> ${info.name}</h5>
                <div class="row">
        `;

        categorySettings.forEach(setting => {
            html += createSettingField(setting);
        });

        html += `
                </div>
            </div>
        `;
    });

    settingsContent.innerHTML = html;
}

/**
 * Erstellt ein Eingabefeld für eine Einstellung
 * @param {Object} setting - Das Einstellungsobjekt
 * @return {string} HTML für das Eingabefeld
 */
function createSettingField(setting) {
    const fieldId = `setting_${setting.key}`;
    let inputHtml = '';
    
    switch (setting.type) {
        case 'boolean':
            inputHtml = `
                <div class="form-check form-switch">
                    <input class="form-check-input" type="checkbox" id="${fieldId}" 
                           ${setting.value === 'true' ? 'checked' : ''} 
                           onchange="updateSetting('${setting.key}', this.checked ? 'true' : 'false')">
                    <label class="form-check-label" for="${fieldId}">
                        ${setting.value === 'true' ? 'Enabled' : 'Disabled'}
                    </label>
                </div>
            `;
            break;
            
        case 'integer':
            inputHtml = `
                <input type="number" class="form-control" id="${fieldId}" 
                       value="${setting.value}" 
                       onchange="updateSetting('${setting.key}', this.value)"
                       onblur="updateSetting('${setting.key}', this.value)">
            `;
            break;
            
        case 'string':
            if (setting.key.includes('password')) {
                inputHtml = `
                    <div class="input-group">
                        <input type="password" class="form-control" id="${fieldId}" 
                               value="${setting.value}" 
                               onchange="updateSetting('${setting.key}', this.value)"
                               onblur="updateSetting('${setting.key}', this.value)">
                        <button class="btn btn-outline-secondary" type="button" 
                                onclick="togglePasswordVisibility('${fieldId}')">
                            <i class="fas fa-eye"></i>
                        </button>
                    </div>
                `;
            } else if (setting.key === 'log_level') {
                const levels = ['debug', 'info', 'warn', 'error'];
                inputHtml = `
                    <select class="form-select" id="${fieldId}" 
                            onchange="updateSetting('${setting.key}', this.value)">
                        ${levels.map(level => 
                            `<option value="${level}" ${setting.value === level ? 'selected' : ''}>${level.toUpperCase()}</option>`
                        ).join('')}
                    </select>
                `;
            } else {
                inputHtml = `
                    <input type="text" class="form-control" id="${fieldId}" 
                           value="${setting.value}" 
                           onchange="updateSetting('${setting.key}', this.value)"
                           onblur="updateSetting('${setting.key}', this.value)">
                `;
            }
            break;
            
        default:
            inputHtml = `
                <input type="text" class="form-control" id="${fieldId}" 
                       value="${setting.value}" 
                       onchange="updateSetting('${setting.key}', this.value)"
                       onblur="updateSetting('${setting.key}', this.value)">
            `;
    }

    return `
        <div class="col-md-6 setting-item">
            <label for="${fieldId}" class="form-label fw-bold">${formatSettingName(setting.key)}</label>
            ${inputHtml}
            <div class="setting-description">${setting.description}</div>
        </div>
    `;
}

/**
 * Formatiert den Setting-Namen für die Anzeige
 * @param {string} key - Der Setting-Key
 * @return {string} Formatierter Name
 */
function formatSettingName(key) {
    return key
        .split('_')
        .map(word => word.charAt(0).toUpperCase() + word.slice(1))
        .join(' ');
}

/**
 * Aktualisiert eine Einstellung auf dem Server
 * @param {string} key - Der Setting-Key
 * @param {string} value - Der neue Wert
 */
function updateSetting(key, value) {
    fetch('/api/settings', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            key: key,
            value: value
        })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        // Zeige Erfolgsmeldung
        showNotification('Setting updated successfully', 'success');
        
        // Log die Änderung
        console.log(`Setting ${key} updated to: ${value}`);
    })
    .catch(error => {
        console.error('Fehler beim Aktualisieren der Einstellung:', error);
        showNotification(`Error updating setting: ${error.message}`, 'error');
        
        // Lade Einstellungen neu, um den ursprünglichen Wert wiederherzustellen
        loadSettings();
    });
}

/**
 * Setzt alle Einstellungen auf Standardwerte zurück
 */
function resetSettings() {
    if (confirm('Möchten Sie wirklich alle Einstellungen auf die Standardwerte zurücksetzen? Diese Aktion kann nicht rückgängig gemacht werden.')) {
        fetch('/api/settings/reset', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        })
        .then(response => {
            if (!response.ok) {
                throw new Error('Network response was not ok');
            }
            return response.json();
        })
        .then(data => {
            showNotification('Settings reset to defaults successfully', 'success');
            loadSettings(); // Lade die Einstellungen neu
        })
        .catch(error => {
            console.error('Fehler beim Zurücksetzen der Einstellungen:', error);
            showNotification(`Error resetting settings: ${error.message}`, 'error');
        });
    }
}

/**
 * Zeigt eine Benachrichtigung an
 * @param {string} message - Die Nachricht
 * @param {string} type - Der Typ (success, error, warning, info)
 */
function showNotification(message, type = 'info') {
    // Erstelle Toast-Container falls nicht vorhanden
    let toastContainer = document.getElementById('toast-container');
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.id = 'toast-container';
        toastContainer.className = 'toast-container position-fixed top-0 end-0 p-3';
        toastContainer.style.zIndex = '1055';
        document.body.appendChild(toastContainer);
    }

    // Erstelle Toast
    const toastId = 'toast-' + Date.now();
    const toastHtml = `
        <div id="${toastId}" class="toast" role="alert" aria-live="assertive" aria-atomic="true">
            <div class="toast-header">
                <i class="fas fa-${type === 'success' ? 'check-circle text-success' : 
                                   type === 'error' ? 'exclamation-circle text-danger' : 
                                   type === 'warning' ? 'exclamation-triangle text-warning' : 
                                   'info-circle text-info'} me-2"></i>
                <strong class="me-auto">Settings</strong>
                <button type="button" class="btn-close" data-bs-dismiss="toast" aria-label="Close"></button>
            </div>
            <div class="toast-body">
                ${message}
            </div>
        </div>
    `;

    toastContainer.insertAdjacentHTML('beforeend', toastHtml);
    
    // Zeige Toast
    const toastElement = document.getElementById(toastId);
    const toast = new bootstrap.Toast(toastElement, { delay: 3000 });
    toast.show();
    
    // Entferne Toast nach dem Ausblenden
    toastElement.addEventListener('hidden.bs.toast', () => {
        toastElement.remove();
    });
}

/**
 * Schaltet die Sichtbarkeit eines Passwort-Feldes um
 * @param {string} fieldId - Die ID des Passwort-Feldes
 */
function togglePasswordVisibility(fieldId) {
    const field = document.getElementById(fieldId);
    const button = field.nextElementSibling.querySelector('i');
    
    if (field.type === 'password') {
        field.type = 'text';
        button.className = 'fas fa-eye-slash';
    } else {
        field.type = 'password';
        button.className = 'fas fa-eye';
    }
}


