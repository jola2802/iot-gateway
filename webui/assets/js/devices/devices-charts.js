const liveCharts = {};
const chartDataCache = new Map(); // Cache für Chart-Daten
const maxDataPoints = 100; // Erhöhte Anzahl für bessere Darstellung

// Funktion zum Öffnen des Chart-Modals
function openChartModal(deviceId, datapoint) {
    const modalTitle = document.querySelector('#modal-chart .modal-title');
    modalTitle.textContent = `Live Chart - ${datapoint.name} (Device: ${deviceId})`;

    // Canvas vorbereiten
    const chartContainer = document.querySelector('#modal-chart .modal-body');
    chartContainer.innerHTML = ''; // Leere Container

    // Erstelle responsive Canvas
    const canvasWrapper = document.createElement('div');
    canvasWrapper.style.cssText = `
        position: relative;
        width: 100%;
        height: 400px;
        margin: 20px 0;
    `;
    
    const newCanvas = document.createElement('canvas');
    newCanvas.style.cssText = `
        width: 100% !important;
        height: 100% !important;
    `;
    canvasWrapper.appendChild(newCanvas);
    chartContainer.appendChild(canvasWrapper);

    const ctx = newCanvas.getContext('2d');

    // Prüfen, ob ein bestehender Chart für diesen Datapoint existiert
    if (liveCharts[datapoint.id]) {
        // Vorhandenen Chart zerstören
        liveCharts[datapoint.id].destroy();
    }

    // Generiere zufällige Farbe für den Chart
    const colors = [
        'rgba(75, 192, 192, 1)',
        'rgba(255, 99, 132, 1)',
        'rgba(54, 162, 235, 1)',
        'rgba(255, 205, 86, 1)',
        'rgba(153, 102, 255, 1)',
        'rgba(255, 159, 64, 1)',
        'rgba(199, 199, 199, 1)',
        'rgba(83, 102, 255, 1)'
    ];
    const randomColor = colors[Math.floor(Math.random() * colors.length)];

    // Neuen Chart erstellen und in liveCharts speichern
    liveCharts[datapoint.id] = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: datapoint.name,
                data: [],
                borderColor: randomColor,
                backgroundColor: randomColor.replace('1)', '0.1)'),
                borderWidth: 3,
                fill: true,
                tension: 0.4,
                pointRadius: 3,
                pointHoverRadius: 6,
                pointBackgroundColor: randomColor,
                pointBorderColor: '#fff',
                pointBorderWidth: 2
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: {
                duration: 300,
                easing: 'easeInOutQuart'
            },
            interaction: {
                intersect: false,
                mode: 'index'
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: {
                        usePointStyle: true,
                        padding: 20,
                        font: {
                            size: 12,
                            weight: 'bold'
                        }
                    }
                },
                tooltip: {
                    backgroundColor: 'rgba(0, 0, 0, 0.8)',
                    titleColor: '#fff',
                    bodyColor: '#fff',
                    borderColor: randomColor,
                    borderWidth: 1,
                    cornerRadius: 8,
                    displayColors: false,
                    callbacks: {
                        title: function(context) {
                            return `Zeit: ${context[0].label}`;
                        },
                        label: function(context) {
                            return `${context.dataset.label}: ${context.parsed.y}`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    title: {
                        display: true,
                        text: 'Zeit',
                        font: {
                            size: 14,
                            weight: 'bold'
                        }
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.1)'
                    }
                },
                y: {
                    title: {
                        display: true,
                        text: 'Wert',
                        font: {
                            size: 14,
                            weight: 'bold'
                        }
                    },
                    grid: {
                        color: 'rgba(0, 0, 0, 0.1)'
                    },
                    beginAtZero: false
                }
            },
            elements: {
                point: {
                    hoverBackgroundColor: randomColor,
                    hoverBorderColor: '#fff'
                }
            }
        }
    });

    // Modal öffnen
    const modal = new bootstrap.Modal(document.getElementById('modal-chart'));
    modal.show();
}

// Funktion zum Aktualisieren des Charts mit verbesserter Performance
function updateChart(datapoint, value, time) {
    if (!liveCharts[datapoint.id]) {
        return;
    }

    const chart = liveCharts[datapoint.id];
    const chartData = chartDataCache.get(datapoint.id) || { labels: [], data: [] };

    // Konvertiere Zeit in lesbares Format
    const timeLabel = time instanceof Date ? time.toLocaleTimeString() : new Date(time).toLocaleTimeString();
    
    // Füge neue Daten hinzu
    chartData.labels.push(timeLabel);
    chartData.data.push(parseFloat(value) || 0);

    // Begrenze die Anzahl der Datenpunkte
    if (chartData.labels.length > maxDataPoints) {
        chartData.labels = chartData.labels.slice(-maxDataPoints);
        chartData.data = chartData.data.slice(-maxDataPoints);
    }

    // Aktualisiere Chart-Daten
    chart.data.labels = chartData.labels;
    chart.data.datasets[0].data = chartData.data;

    // Cache aktualisieren
    chartDataCache.set(datapoint.id, chartData);

    // Chart aktualisieren mit optimierter Animation
    chart.update('none'); // Keine Animation für bessere Performance
}

// Funktion zum Bereinigen von nicht mehr verwendeten Charts
function cleanupCharts() {
    const activeDatapoints = new Set();
    
    // Sammle alle aktiven Datapoints
    document.querySelectorAll('[data-datapoint-id]').forEach(element => {
        const datapointId = element.getAttribute('data-datapoint-id');
        if (datapointId) {
            activeDatapoints.add(datapointId);
        }
    });

    // Entferne Charts für nicht mehr vorhandene Datapoints
    Object.keys(liveCharts).forEach(datapointId => {
        if (!activeDatapoints.has(datapointId)) {
            if (liveCharts[datapointId]) {
                liveCharts[datapointId].destroy();
                delete liveCharts[datapointId];
            }
            chartDataCache.delete(datapointId);
        }
    });
}

// Funktion zum Exportieren von Chart-Daten
function exportChartData(datapointId) {
    const chartData = chartDataCache.get(datapointId);
    if (!chartData) {
        console.warn('Keine Daten für Export verfügbar');
        return;
    }

    const csvContent = [
        'Zeit,Wert',
        ...chartData.labels.map((label, index) => 
            `${label.toLocaleString()},${chartData.data[index]}`
        )
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `chart_data_${datapointId}_${new Date().toISOString().slice(0, 10)}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
}

// Funktion zum Zurücksetzen eines Charts
function resetChart(datapointId) {
    if (liveCharts[datapointId]) {
        liveCharts[datapointId].data.labels = [];
        liveCharts[datapointId].data.datasets[0].data = [];
        liveCharts[datapointId].update();
        chartDataCache.delete(datapointId);
    }
}

// Automatische Bereinigung alle 30 Sekunden
setInterval(cleanupCharts, 30000);

// Event-Listener für Chart-Modal-Schließung
document.addEventListener('DOMContentLoaded', function() {
    const chartModal = document.getElementById('modal-chart');
    if (chartModal) {
        chartModal.addEventListener('hidden.bs.modal', function() {
            // Optional: Charts pausieren wenn Modal geschlossen ist
            Object.values(liveCharts).forEach(chart => {
                if (chart && chart.options) {
                    chart.options.animation = false;
                }
            });
        });
    }
});