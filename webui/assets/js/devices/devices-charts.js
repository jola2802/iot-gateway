const liveCharts = {};

// Funktion zum Öffnen des Chart-Modals
function openChartModal(deviceId, datapoint) {
    const modalTitle = document.querySelector('#modal-chart .modal-title');
    modalTitle.textContent = `Live Chart - ${datapoint.name} (Device: ${deviceId})`;

    // Canvas vorbereiten
    const chartContainer = document.querySelector('#modal-chart .modal-body');
    const existingCanvas = chartContainer.querySelector('canvas');
    if (existingCanvas) {
        existingCanvas.remove(); // Altes Canvas entfernen
    }

    const newCanvas = document.createElement('canvas');
    chartContainer.appendChild(newCanvas);

    const ctx = newCanvas.getContext('2d');

    // Prüfen, ob ein bestehender Chart für diesen Datapoint existiert
    if (liveCharts[datapoint.id]) {
        // Vorhandenen Chart zerstören
        liveCharts[datapoint.id].destroy();
    }

    // Neuen Chart erstellen und in liveCharts speichern
    liveCharts[datapoint.id] = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: datapoint.name,
                data: [],
                borderColor: 'rgba(75, 192, 192, 1)',
                borderWidth: 3,
                fill: false
            }]
        },
        options: {
            responsive: true,
            animation: {
                duration: 0,
                easing: 'linear'
            },
            scales: {
                x: {
                    title: {
                        display: true,
                        text: 'Time'
                    }
                },
                y: {
                    title: {
                        display: true,
                        text: 'Value'
                    }
                }
            }
        }
    });
    // Modal öffnen
    const modal = new bootstrap.Modal(document.getElementById('modal-chart'));
    modal.show();
}

// Funktion zum Aktualisieren des Charts
function updateChart(datapoint, value, time) {
    if (!liveCharts[datapoint.id]) {
        return;
    }

    // Aktualisieren des Charts mit den neuen Daten
    liveCharts[datapoint.id].data.labels.push(time.toLocaleTimeString());
    liveCharts[datapoint.id].data.datasets[0].data.push(value);

    // Begrenzen Sie die Anzahl der Datenpunkte auf die letzten 50
    const maxDataPoints = 50;
    if (liveCharts[datapoint.id].data.labels.length > maxDataPoints) {
        liveCharts[datapoint.id].data.labels = liveCharts[datapoint.id].data.labels.slice(-maxDataPoints);
        liveCharts[datapoint.id].data.datasets[0].data = liveCharts[datapoint.id].data.datasets[0].data.slice(-maxDataPoints);
    }

    liveCharts[datapoint.id].update();
}