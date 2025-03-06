// Event Listener für das Laden der Seite
// document.addEventListener('DOMContentLoaded', loadSettings);

// Checkbox Event Listener
document.getElementById('use-custom-services').addEventListener('change', function(e) {
    const container = document.getElementById('custom-services-container');
    const inputs = container.querySelectorAll('input');
    container.style.display = e.target.checked ? 'block' : 'none';
    inputs.forEach(input => input.disabled = !e.target.checked);
});

document.getElementById('use-external-broker').addEventListener('change', function(e) {
    const container = document.getElementById('external-broker-container');
    const inputs = container.querySelectorAll('input');
    container.style.display = e.target.checked ? 'block' : 'none';
    inputs.forEach(input => input.disabled = !e.target.checked);
});

// Laden der Einstellungen
async function loadSettings() {
    try {
        const response = await fetch('/api/settings');
        const settings = await response.json();

        // Setze die Werte in die Formularfelder
        document.getElementById('docker-ip').value = settings.docker_ip || '';

        // Externe Dienste
        if (settings.use_custom_services) {
            document.getElementById('use-custom-services').checked = true;
            document.getElementById('custom-services-container').style.display = 'block';
            document.getElementById('nodered-url').value = settings.nodered_url || '';
            document.getElementById('nodered-url').disabled = false;
            document.getElementById('influxdb-url').value = settings.influxdb_url || '';
            document.getElementById('influxdb-url').disabled = false;
        }

        // Externer Broker
        if (settings.use_external_broker) {
            document.getElementById('use-external-broker').checked = true;
            document.getElementById('external-broker-container').style.display = 'block';
            document.getElementById('broker-url').value = settings.broker_url || '';
            document.getElementById('broker-url').disabled = false;
            document.getElementById('broker-port').value = settings.broker_port || '';
            document.getElementById('broker-port').disabled = false;
            document.getElementById('broker-username').value = settings.broker_username || '';
            document.getElementById('broker-username').disabled = false;
            document.getElementById('broker-password').value = settings.broker_password || '';
            document.getElementById('broker-password').disabled = false;
        }
    } catch (error) {
        console.error('Fehler beim Laden der Einstellungen:', error);
        alert('Fehler beim Laden der Einstellungen!');
    }
}

// Speichern der Einstellungen
document.getElementById('settings-form').addEventListener('submit', async (event) => {
    event.preventDefault();

    const useCustomServices = document.getElementById('use-custom-services').checked;
    const useExternalBroker = document.getElementById('use-external-broker').checked;

    const settings = {
        docker_ip: document.getElementById('docker-ip').value,
        use_custom_services: useCustomServices,
        use_external_broker: useExternalBroker
    };

    // Füge die benutzerdefinierten Dienst-URLs hinzu, wenn aktiviert
    if (useCustomServices) {
        settings.nodered_url = document.getElementById('nodered-url').value;
        settings.influxdb_url = document.getElementById('influxdb-url').value;
    }

    // Füge die externen Broker-Einstellungen hinzu, wenn aktiviert
    if (useExternalBroker) {
        settings.broker_url = document.getElementById('broker-url').value;
        settings.broker_port = document.getElementById('broker-port').value;
        settings.broker_username = document.getElementById('broker-username').value;
        settings.broker_password = document.getElementById('broker-password').value;
    }

    try {
        const response = await fetch('/api/settings', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(settings)
        });

        if (response.ok) {
            alert('Settings saved successfully!');
        } else {
            const error = await response.json();
            alert('Error saving settings: ' + error.message);
        }
    } catch (error) {
        console.error('Error:', error);
        alert('Error saving settings!');
    }
});
