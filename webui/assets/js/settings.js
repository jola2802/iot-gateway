document.addEventListener('DOMContentLoaded', function() {
    
    // Initialisiere die Filter für die Log-Levels
    initializeLogFilters();
    
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


