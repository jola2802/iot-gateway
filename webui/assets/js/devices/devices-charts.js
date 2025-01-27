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

    // Neuen Chart erstellen
    liveCharts[datapoint.id] = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [], // Labels werden dynamisch hinzugefügt
            datasets: [{
                label: datapoint.name,
                data: [], // Daten werden dynamisch hinzugefügt
                borderColor: 'rgba(75, 192, 192, 1)',
                borderWidth: 2,
                fill: false
            }]
        },
        options: {
            responsive: true,
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
function updateChart(chart, newValue) {
    if (!chart) {
        console.warn('Chart-Instanz ist undefined');
        return;
    }

    const currentTime = new Date().toLocaleTimeString();
    chart.data.labels.push(currentTime);
    chart.data.datasets[0].data.push(newValue);

    if (chart.data.labels.length > 20) {
        chart.data.labels.shift();
        chart.data.datasets[0].data.shift();
    }

    try {
        chart.update('none'); // 'none' verhindert eine Animation
    } catch (error) {
        console.error('Fehler beim Aktualisieren des Charts:', error);
    }
}