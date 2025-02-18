// Lade die aktuellen Einstellungen beim Seitenstart
document.addEventListener('DOMContentLoaded', loadSettings);

async function loadSettings() {
    try {
        const response = await fetch('/api/settings');
        const config = await response.json();

        console.log(config);
        
        // Setze Web UI Einstellungen
        document.getElementById('http-port').value = config.webui.http_port;
        document.getElementById('https-port').value = config.webui.https_port;
        document.getElementById('use-https').value = config.webui.use_https.toString();

        // Füge Listener hinzu
        const container = document.getElementById('listeners-container');
        container.innerHTML = ''; // Lösche bestehende Listener
        
        config.listeners.forEach(listener => {
            addListener(listener);
        });
    } catch (error) {
        console.error('Fehler beim Laden der Einstellungen:', error);
        alert('Fehler beim Laden der Einstellungen!');
    }
}

function addListener(data = {}) {
    const template = document.getElementById('listener-template');
    const container = document.getElementById('listeners-container');
    const clone = template.content.cloneNode(true);
    
    if (data.id) {
        clone.querySelector('.listener-id').value = data.id;
        clone.querySelector('.listener-address').value = data.address;
        clone.querySelector('.listener-type').value = data.type;
        clone.querySelector('.listener-tls').value = (data.tls || false).toString();
    }
    
    clone.querySelector('.remove-listener').addEventListener('click', function(e) {
        e.target.closest('.listener-entry').remove();
    });
    
    container.appendChild(clone);
}

document.getElementById('settings-form').addEventListener('submit', async (event) => {
    event.preventDefault();

    const config = {
        webui: {
            http_port: document.getElementById('http-port').value,
            https_port: document.getElementById('https-port').value,
            use_https: document.getElementById('use-https').value === 'true',
            tls_cert: "server.crt", // Behalte Standardwerte bei
            tls_key: "server.key"
        },
        listeners: []
    };

    // Sammle alle Listener-Einstellungen
    document.querySelectorAll('.listener-entry').forEach(entry => {
        config.listeners.push({
            id: entry.querySelector('.listener-id').value,
            address: entry.querySelector('.listener-address').value,
            type: entry.querySelector('.listener-type').value,
            tls: entry.querySelector('.listener-tls').value === 'true'
        });
    });

    try {
        const response = await fetch('/api/settings', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(config)
        });

        if (response.ok) {
            alert('Einstellungen wurden gespeichert! Bitte starten Sie den Container neu, damit die Änderungen wirksam werden.');
        } else {
            alert('Fehler beim Speichern der Einstellungen!');
        }
    } catch (error) {
        console.error('Fehler:', error);
        alert('Fehler beim Speichern der Einstellungen!');
    }
});

// Button zum Hinzufügen neuer Listener
document.getElementById('add-listener').addEventListener('click', () => {
    addListener();
});
