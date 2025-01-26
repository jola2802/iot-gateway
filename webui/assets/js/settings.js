document.getElementById('settings-form').addEventListener('submit', (event) => {
    event.preventDefault();

    const nodeRedUrl = document.getElementById('node-red-url').value;
    const influxdbUrl = document.getElementById('influxdb-url').value;

    // Beispiel für eine API-Anfrage oder lokale Speicherung
    console.log('Node-RED URL:', nodeRedUrl);
    console.log('InfluxDB URL:', influxdbUrl);

    // Zeige eine Erfolgsmeldung
    alert('Settings saved successfully!');
});
