const form = document.getElementById('data-query-form');
const dataTableBody = document.getElementById('data-table-body');
const ctx = document.getElementById('data-chart').getContext('2d');
const deviceSelect = document.getElementById('device-select');
const datapointSelect = document.getElementById('datapoint-select');
const exportButton = document.getElementById('export-btn');
const dataSummary = document.getElementById('data-summary');
const dataCount = document.getElementById('data-count');
const tableContainer = document.getElementById('table-container');
const showTableToggle = document.getElementById('auto-refresh');
const prevPageBtn = document.getElementById('prev-page');
const nextPageBtn = document.getElementById('next-page');
const pageInfo = document.getElementById('page-info');

let chart;
let allData = []; // Alle Daten speichern
let currentPage = 0;
const itemsPerPage = 50;

// Funktion: Lade die Geräte und fülle das Dropdown
async function loadDevices() {
    try {
        const response = await fetch('/api/getDevices'); // Endpunkt für Geräte
        if (!response.ok) {
            throw new Error('Failed to fetch devices');
        }
        const data = await response.json();
        const devices = data.devices; // Erwarte ein Array von Geräten

        // Fülle das Geräte-Dropdown-Menü
        deviceSelect.innerHTML = devices.map(
            (device) => `<option value="${device.id}">${device.deviceName}</option>`
        ).join('');
        deviceSelect.insertAdjacentHTML('afterbegin', '<option value="" disabled selected>Select a Device</option>');
    } catch (error) {
        console.error('Error fetching devices:', error);
        alert('Failed to load devices.');
    }
}

// Funktion: Lade die Measurements basierend auf dem ausgewählten Gerät
async function loadMeasurementsForDevice(id) {
    try {
        const response = await fetch('/api/get-measurements', { // Endpunkt für Measurements
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ deviceId: id }), // deviceId wird als JSON-Body übergeben
        });

        if (!response.ok) {
            throw new Error('Failed to fetch measurements');
        }

        const data = await response.json();
        const measurements = data.measurements; // Erwarte ein Array von Measurements

        // console.log('Measurements:', measurements);
        // Fülle das Measurements-Dropdown-Menü
        datapointSelect.innerHTML = measurements.map(
            (measurement) => `<option value="${measurement}">${measurement}</option>`
        ).join('');
        datapointSelect.insertAdjacentHTML('afterbegin', '<option value="" disabled selected>Select a Datapoint</option>');
    } catch (error) {
        console.error('Error fetching measurements:', error);
        alert('Failed to load measurements.');
    }
}

// Funktionen für Paginierung
function updatePagination() {
    const totalPages = Math.ceil(allData.length / itemsPerPage);
    prevPageBtn.disabled = currentPage === 0;
    nextPageBtn.disabled = currentPage >= totalPages - 1;
    pageInfo.textContent = totalPages > 0 ? `${currentPage + 1} / ${totalPages}` : '0 / 0';
}

function displayCurrentPage() {
    const startIdx = currentPage * itemsPerPage;
    const endIdx = Math.min(startIdx + itemsPerPage, allData.length);
    const pageData = allData.slice(startIdx, endIdx);
    
    if (pageData.length === 0) {
        dataTableBody.innerHTML = `
            <tr>
                <td colspan="2" class="text-center text-muted py-4">
                    <i class="fas fa-search me-2"></i>No data found
                </td>
            </tr>
        `;
        return;
    }
    
    dataTableBody.innerHTML = pageData.map((item) =>
        `<tr>
            <td class="text-center font-monospace small">${new Date(item.x).toLocaleString('de-DE')}</td>
            <td class="text-center"><span class="badge bg-light text-dark">${Number(item.y).toFixed(3)}</span></td>
        </tr>`
    ).join('');
}

function calculateStatistics(data) {
    if (data.length === 0) return null;
    
    const values = data.map(item => Number(item.y)).filter(val => !isNaN(val));
    const min = Math.min(...values);
    const max = Math.max(...values);
    const avg = values.reduce((a, b) => a + b, 0) / values.length;
    const first = new Date(data[0].x);
    const last = new Date(data[data.length - 1].x);
    const duration = (last - first) / 1000 / 60; // Minuten
    
    return { min, max, avg, count: values.length, duration, first, last };
}

function updateDataSummary(stats) {
    if (!stats) {
        dataSummary.innerHTML = '<p class="text-muted">No data available</p>';
        return;
    }
    
    dataSummary.innerHTML = `
        <div class="row g-2">
            <div class="col-6">
                <div class="text-center p-2 bg-light rounded">
                    <div class="h6 mb-0 text-primary">${stats.count}</div>
                    <small class="text-muted">Records</small>
                </div>
            </div>
            <div class="col-6">
                <div class="text-center p-2 bg-light rounded">
                    <div class="h6 mb-0 text-success">${stats.duration.toFixed(1)}m</div>
                    <small class="text-muted">Duration</small>
                </div>
            </div>
            <div class="col-6">
                <div class="text-center p-2 bg-light rounded">
                    <div class="h6 mb-0 text-danger">${stats.max.toFixed(3)}</div>
                    <small class="text-muted">Maximum</small>
                </div>
            </div>
            <div class="col-6">
                <div class="text-center p-2 bg-light rounded">
                    <div class="h6 mb-0 text-info">${stats.min.toFixed(3)}</div>
                    <small class="text-muted">Minimum</small>
                </div>
            </div>
            <div class="col-12">
                <div class="text-center p-2 bg-light rounded">
                    <div class="h6 mb-0 text-warning">${stats.avg.toFixed(3)}</div>
                    <small class="text-muted">Average</small>
                </div>
            </div>
        </div>
        <hr>
        <div class="small text-muted">
            <div><i class="fas fa-clock me-1"></i><strong>From:</strong> ${stats.first.toLocaleString('de-DE')}</div>
            <div><i class="fas fa-clock me-1"></i><strong>To:</strong> ${stats.last.toLocaleString('de-DE')}</div>
        </div>
    `;
}

// Event-Listener für Paginierung
prevPageBtn.addEventListener('click', () => {
    if (currentPage > 0) {
        currentPage--;
        displayCurrentPage();
        updatePagination();
    }
});

nextPageBtn.addEventListener('click', () => {
    const totalPages = Math.ceil(allData.length / itemsPerPage);
    if (currentPage < totalPages - 1) {
        currentPage++;
        displayCurrentPage();
        updatePagination();
    }
});

// Event-Listener für Tabelle ein-/ausblenden
showTableToggle.addEventListener('change', () => {
    tableContainer.style.display = showTableToggle.checked ? 'block' : 'none';
});

// Event-Listener für Gerätewechsel
deviceSelect.addEventListener('change', (event) => {
    const selectedDevice = event.target.value;
    datapointSelect.innerHTML = '<option value="" disabled selected>Loading...</option>'; // Ladehinweis
    loadMeasurementsForDevice(selectedDevice);
});

// Event-Listener für das Formular
form.addEventListener('submit', async (event) => {
    event.preventDefault();

    const startTime = document.getElementById('start-time').value;
    const duration = document.getElementById('duration').value;
    const selectedMeasurement = datapointSelect.value; // Ausgewähltes Measurement
    const selectedDevice = deviceSelect.value; // Ausgewähltes Device

    // Fetch data from InfluxDB
    try {
        const response = await fetch('/api/query-data', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                start: startTime,
                duration: duration,
                measurement: selectedMeasurement,
            }),
        });

        if (!response.ok) {
            throw new Error('Failed to fetch data');
        }

        const data = await response.json();

        console.log('Data:', data);

        // Aktualisiere globale Daten
        allData = data;
        currentPage = 0;

        // Aktualisiere Counter
        dataCount.textContent = `${data.length} records`;

        // Berechne und zeige Statistiken
        const stats = calculateStatistics(data);
        updateDataSummary(stats);

        // Zeige erste Seite der Tabelle
        displayCurrentPage();
        updatePagination();

        const timestamps = data.map((item) => new Date(item.x)); // Konvertiere Zeitstempel in Date-Objekte
        const values = data.map((item) => item.y);

        // Update Chart
        // Destroy existing chart if it exists
        if (chart) {
            chart.destroy();
        }
        chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: timestamps,
                datasets: [{
                    label: 'Values',
                    data: values,
                    borderColor: 'blue',
                    fill: false,
                }],
            },
            options: {
                responsive: true,
                plugins: {
                    legend: {
                        display: true,
                    },
                },
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            tooltipFormat: 'HH:mm:ss',
                            displayFormats: {
                                minute: 'HH:mm:ss',
                                hour: 'HH:mm:ss',
                            },
                            title: {
                                display: true,
                                text: 'Time',
                            },
                        },
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'Value',
                        },
                    },
                },
            },
        });      
    } catch (error) {
        console.error('Error fetching data:', error);
        alert('Failed to fetch data from InfluxDB.');
    }
});

// Event-Listener für den Export-Button
exportButton.addEventListener('click', () => {
    if (allData.length === 0) {
        alert('No data to export');
        return;
    }
    exportToCSV(allData);
});


// Funktion: Daten als CSV herunterladen
function exportToCSV(data) {
    const headers = ['Timestamp', 'Value'];
    const csvRows = [headers.join(',')];

    data.forEach((item) => {
        const row = [new Date(item.x).toLocaleString(), item.y];
        csvRows.push(row.join(','));
    });

    const blob = new Blob([csvRows.join('\n')], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);

    const a = document.createElement('a');
    a.href = url;
    a.download = 'data.csv';
    a.click();
    URL.revokeObjectURL(url);
}

// Funktion: Daten als PDF herunterladen
function exportToPDF(data) {
    const pdf = new jsPDF();
    pdf.text('Exported Data', 10, 10);

    const tableData = data.map((item) => [new Date(item.x).toLocaleString(), item.y]);

    pdf.autoTable({
        head: [['Timestamp', 'Value']],
        body: tableData,
    });

    pdf.save('data.pdf');
}



// Lade Geräte, wenn die Seite geladen wird
document.addEventListener('DOMContentLoaded', loadDevices);