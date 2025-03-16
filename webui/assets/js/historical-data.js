const form = document.getElementById('data-query-form');
const dataTableBody = document.getElementById('data-table-body');
const ctx = document.getElementById('data-chart').getContext('2d');
const deviceSelect = document.getElementById('device-select');
const datapointSelect = document.getElementById('datapoint-select');
const exportButton = document.createElement('button'); // Export-Button erstellen
// Tooltip for export button
exportButton.setAttribute('data-bs-toggle', 'tooltip');
exportButton.setAttribute('data-bs-placement', 'top');
exportButton.setAttribute('title', 'Export data as CSV');
let chart;

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

        const timestamps = data.map((item) => new Date(item.x)); // Konvertiere Zeitstempel in Date-Objekte
        const values = data.map((item) => item.y);

        
        dataTableBody.innerHTML = data.map((item) =>
            `<tr>
                <td>${new Date(item.x).toLocaleString()}</td>
                <td>${item.y}</td>
            </tr>`
        ).join('');

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

// Füge den Export-Button zum chart-Container hinzu
exportButton.textContent = 'Export Data';
exportButton.className = 'btn btn-secondary btn-sm mt-3';

// Füge den Button in den Container mit der ID "chart-container" ein
const chartContainer = document.getElementById('chart-container');
if (chartContainer) {
    chartContainer.appendChild(exportButton);
} else {
    console.error('Container #chart-container not found!');
}


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

// Event-Listener für den Export-Button
exportButton.addEventListener('click', () => {
    exportToCSV(chart.data.datasets[0].data.map((value, index) => ({
        x: chart.data.labels[index],
        y: value,
    })));
});

// Lade Geräte, wenn die Seite geladen wird
document.addEventListener('DOMContentLoaded', loadDevices);