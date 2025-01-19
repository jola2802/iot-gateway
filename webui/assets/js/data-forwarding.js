async function fetchAndPopulateDataForwarding() {
    try {
        const response = await fetch(`${BASE_PATH}/getDataForwarding`);
        if (!response.ok) {
            throw new Error(`Error fetching /getDataForwarding: ${response.status}`);
        }

        const dataForwarding = await response.json();
        const tableBody = document.querySelector('#table-data-forwarding tbody');
        tableBody.innerHTML = ''; // Tabelle leeren

        dataForwarding.forEach(route => {
            const row = document.createElement('tr');

            // Route ID
            const routeIdCell = document.createElement('td');
            routeIdCell.textContent = route.route_id;
            row.appendChild(routeIdCell);

            // Type
            const typeCell = document.createElement('td');
            typeCell.textContent = route.type;
            row.appendChild(typeCell);

            // Devices
            const devicesCell = document.createElement('td');
            devicesCell.textContent = route.devices.join(', ');
            row.appendChild(devicesCell);

            // Address
            const addressCell = document.createElement('td');
            addressCell.textContent = route.address;
            row.appendChild(addressCell);

            // Last Send
            const lastSendCell = document.createElement('td');
            lastSendCell.textContent = route.last_send || 'N/A';
            row.appendChild(lastSendCell);

            // Actions
            const actionsCell = document.createElement('td');
            actionsCell.innerHTML = `
                <a class="btn btnMaterial btn-flat success semicircle" href="#">
                    <i class="fas fa-pen" data-bs-toggle="modal" data-bs-target="#modal-new-route"></i>
                </a>
                <a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" style="margin-left: 5px;" href="#">
                    <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                </a>
            `;
            row.appendChild(actionsCell);

            tableBody.appendChild(row);
        });
    } catch (error) {
        console.error('Error fetching /getDataForwarding:', error);
    }
}

async function fetchAndPopulateImgProcesses() {
    try {
        const response = await fetch(`${BASE_PATH}/getImgProcesses`);
        if (!response.ok) {
            throw new Error(`Error fetching /getImgProcesses: ${response.status}`);
        }

        const imgProcesses = await response.json();
        const tableBody = document.querySelector('#table-image-capture-process tbody');
        tableBody.innerHTML = ''; // Tabelle leeren

        imgProcesses.forEach(process => {
            const row = document.createElement('tr');

            // Process ID
            const processIdCell = document.createElement('td');
            processIdCell.textContent = process.process_id;
            row.appendChild(processIdCell);

            // Device
            const deviceCell = document.createElement('td');
            deviceCell.textContent = process.device;
            row.appendChild(deviceCell);

            // Address
            const addressCell = document.createElement('td');
            addressCell.textContent = process.address;
            row.appendChild(addressCell);

            // Last Capture
            const lastCaptureCell = document.createElement('td');
            lastCaptureCell.textContent = process.last_capture || 'N/A';
            row.appendChild(lastCaptureCell);

            // Actions
            const actionsCell = document.createElement('td');
            actionsCell.innerHTML = `
                <a class="btn btnMaterial btn-flat success semicircle" href="#">
                    <i class="fas fa-pen" data-bs-toggle="modal" data-bs-target="#modal-new-img-process"></i>
                </a>
                <a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" style="margin-left: 5px;" href="#">
                    <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                </a>
            `;
            row.appendChild(actionsCell);

            tableBody.appendChild(row);
        });
    } catch (error) {
        console.error('Error fetching /getImgProcesses:', error);
    }
}

fetchAndPopulateDataForwarding();
fetchAndPopulateImgProcesses();

// ##################

const destinationTypeSelect = document.getElementById('select-destination-type');

destinationTypeSelect.addEventListener('change', () => {
    const selectedValue = destinationTypeSelect.value;
    
    hideAllConfigsRoute();
    
    switch (selectedValue) {
        case 'rest': // REST
            restConfig.style.display = 'block';
            break;
        case 'file-based': // FILE
            fileConfig.style.display = 'block';
            restConfig.style.display = 'none';
            break;
        case 'mqtt': // MQTT
        //     mqttConfig.style.display = 'block';
        //     restConfig.style.display = 'none';
        //     break;
        default: // Keine oder ungültige Auswahl
            // Alle Karten bleiben ausgeblendet
            break;
    }
})

const restConfig = document.getElementById('rest-config');
const fileConfig = document.getElementById('file-config');
// const mqttConfig = document.getElementById('mqtt-config');

function hideAllConfigsRoute() {
    // restConfig.style.display = 'none';
    fileConfig.style.display = 'none';
    // mqttConfig.style.display = 'none';
}

hideAllConfigsRoute();

async function fetchAndPopulateDevicesProcess() {
    try {
        const response = await fetch(`${BASE_PATH}/getDevices`);
        if (!response.ok) {
            throw new Error(`Error fetching /getDevices: ${response.status}`);
        }

        const devices = await response.json();
        const selectDeviceElement = document.getElementById('select-device');

        // Aktuelle Optionen löschen
        selectDeviceElement.innerHTML = '';

        // Standardoption hinzufügen
        const defaultOption = document.createElement('option');
        defaultOption.value = '';
        // defaultOption.textContent = 'Select a device';
        selectDeviceElement.appendChild(defaultOption);

        // Geräte hinzufügen
        devices.forEach(device => {
            const option = document.createElement('option');
            option.value = device.device_id; // Verwende die Geräte-ID als Wert
            option.textContent = `${device.device_id} - ${device.device || 'Unnamed Device'}`;
            selectDeviceElement.appendChild(option);
        });
    } catch (error) {
        console.error('Error fetching /getDevices:', error);
    }
}

//Adding Header in List for new-image-process
document.getElementById('add-header-button-p').addEventListener('click', function () {
    // Eingabefelder referenzieren
    const headerKeyInput = document.getElementById('header-key');
    const headerValueInput = document.getElementById('header-value');
    const headerList = document.getElementById('header-list');

    // Werte aus den Eingabefeldern holen
    const key = headerKeyInput.value.trim();
    const value = headerValueInput.value.trim();

    // Prüfen, ob beide Felder ausgefüllt sind
    if (!key || !value) {
        alert('Bitte geben Sie sowohl einen Header Key als auch einen Header Value ein.');
        return;
    }

    // Neues Listenelement erstellen
    const listItem = document.createElement('li');
    listItem.classList.add('list-group-item', 'd-flex', 'justify-content-between', 'align-items-center');

    // Inhalt des Listenelements hinzufügen
    listItem.innerHTML = `
        <span><strong>${key}</strong>: ${value}</span>
        <button class="btn btn-danger btn-sm remove-header-button" type="button">Remove</button>
    `;

    // Löschen-Button-Event hinzufügen
    const deleteButton = listItem.querySelector('.remove-header-button');
    deleteButton.addEventListener('click', () => {
        headerList.removeChild(listItem);
    });

    // Neues Listenelement zur Liste hinzufügen
    headerList.appendChild(listItem);

    // Eingabefelder leeren
    headerKeyInput.value = '';
    headerValueInput.value = '';
});

//Adding Header in List for new-route-modal
document.getElementById('add-header-button-r').addEventListener('click', function () {
    // Eingabefelder referenzieren
    const headerKeyInput = document.getElementById('header-key-r');
    const headerValueInput = document.getElementById('header-value-r');
    const headerList = document.getElementById('header-list-r');

    // Werte aus den Eingabefeldern holen
    const key = headerKeyInput.value.trim();
    const value = headerValueInput.value.trim();

    // Prüfen, ob beide Felder ausgefüllt sind
    if (!key || !value) {
        alert('Bitte geben Sie sowohl einen Header Key als auch einen Header Value ein.');
        return;
    }

    // Neues Listenelement erstellen
    const listItem = document.createElement('li');
    listItem.classList.add('list-group-item', 'd-flex', 'justify-content-between', 'align-items-center');

    // Inhalt des Listenelements hinzufügen
    listItem.innerHTML = `
        <span><strong>${key}</strong>: ${value}</span>
        <button class="btn btn-danger btn-sm remove-header-button" type="button">Remove</button>
    `;

    // Löschen-Button-Event hinzufügen
    const deleteButton = listItem.querySelector('.remove-header-button');
    deleteButton.addEventListener('click', () => {
        headerList.removeChild(listItem);
    });

    // Neues Listenelement zur Liste hinzufügen
    headerList.appendChild(listItem);

    // Eingabefelder leeren
    headerKeyInput.value = '';
    headerValueInput.value = '';
});

const newImgProcessModal = document.getElementById('modal-new-img-process');
newImgProcessModal.addEventListener('show.bs.modal', fetchAndPopulateDevicesProcess);

// ######################
// Selektiere das Modal
var modal = document.getElementById('modal-new-route');

// Füge einen Event Listener hinzu, der ausgelöst wird, wenn das Modal gezeigt wird
modal.addEventListener('show.bs.modal', function (event) {
    // ===============================
    // 1. Optionen für "Destination Type" hinzufügen
    // ===============================
    var destinationTypeSelect = document.getElementById('select-destination-type');
    // Leere bestehende Optionen
    destinationTypeSelect.innerHTML = '';

    // Erstelle eine neue Optgroup
    var destOptgroup = document.createElement('optgroup');

    // Definiere die neuen Optionen
    var destinationTypes = [
        { value: 'rest', text: 'REST' },
        { value: 'file-based', text: 'File-based' },
        { value: 'mqtt', text: 'MQTT' }
    ];

    // Füge jede Option zur Optgroup hinzu
    destinationTypes.forEach(function(type) {
        var option = document.createElement('option');
        option.value = type.value;
        option.text = type.text;
        destOptgroup.appendChild(option);
    });

    // Füge die Optgroup dem Select-Element hinzu
    destinationTypeSelect.appendChild(destOptgroup);

    // ===============================
    // 2. Optionen für "Data Format" hinzufügen
    // ===============================
    var dataFormatSelect = document.getElementById('select-data-format');
    // Leere bestehende Optionen
    dataFormatSelect.innerHTML = '';

    // Erstelle eine neue Optgroup
    var formatOptgroup = document.createElement('optgroup');

    // Definiere die neuen Optionen
    var dataFormats = [
        { value: 'json', text: 'JSON' },
        { value: 'xml', text: 'XML' }
    ];

    // Füge jede Option zur Optgroup hinzu
    dataFormats.forEach(function(format) {
        var option = document.createElement('option');
        option.value = format.value;
        option.text = format.text;
        formatOptgroup.appendChild(option);
    });

    // Füge die Optgroup dem Select-Element hinzu
    dataFormatSelect.appendChild(formatOptgroup);

    // ===============================
    // 3. Geräte abrufen und zur Liste hinzufügen
    // ===============================
    fetch(`${BASE_PATH}/getDevices`)
        .then(function(response) {
            if (!response.ok) {
                throw new Error('Netzwerkantwort war nicht ok');
            }
            return response.json();
        })
        .then(function(devices) {
            var deviceList = document.getElementById('check-list-devices');
            // Leere die bestehende Liste
            deviceList.innerHTML = '';

            // Überprüfe, ob devices ein Array ist
            if (Array.isArray(devices)) {
                devices.forEach(function(device) {
                    // Erstelle die Listengruppelemente
                    var li = document.createElement('li');
                    li.className = 'list-group-item';

                    var div = document.createElement('div');
                    div.className = 'form-check';

                    var input = document.createElement('input');
                    input.type = 'checkbox';
                    input.className = 'form-check-input';
                    input.id = 'formCheck-' + device.device_id;
                    input.value = device.device; // Optional: Setze den Wert auf die Geräte-ID

                    var label = document.createElement('label');
                    label.className = 'form-check-label';
                    label.htmlFor = 'formCheck-' + device.device_id;
                    label.textContent = device.device;

                    // Füge Input und Label zum Div hinzu
                    div.appendChild(input);
                    div.appendChild(label);

                    // Füge das Div zum Listenelement hinzu
                    li.appendChild(div);

                    // Füge das Listenelement zur Liste hinzu
                    deviceList.appendChild(li);
                });
            } else {
                console.error('Unerwartetes Datenformat:', devices);
            }
        })
        .catch(function(error) {
            console.error('Fehler beim Abrufen der Geräte:', error);
        });
});

// ************************
// Browse for node
// ************************

// Selektiere alle Buttons mit der Klasse 'btn-open-browsed-nodes'
const lupeButtons = document.querySelectorAll('.btn-open-browsed-nodes');

lupeButtons.forEach(function(button) {
    button.addEventListener('click', function(event) {
        event.preventDefault();

        // Initialisiere das 'modal-browsed-nodes' Modal
        const browsedNodesModalElement = document.getElementById('modal-browsed-nodes');
        const browsedNodesModal = new bootstrap.Modal(browsedNodesModalElement, {
            backdrop: 'static', // Verhindert das Schließen durch Klick auf den Hintergrund
            keyboard: false     // Verhindert das Schließen durch Drücken der Escape-Taste
        });

        // Zeige das Modal
        browsedNodesModal.show();

        // Anpassen der Z-Index für gestapelte Modals
        browsedNodesModalElement.style.zIndex = '2055'; // Höher als das Standard-Modal (1050)

        // Finden oder Erstellen eines Backdrops für das neue Modal
        let existingBackdrop = document.querySelector('.modal-backdrop.show');
        if (existingBackdrop) {
            // Wenn bereits ein Backdrop vorhanden ist, erhöhen Sie dessen z-index
            existingBackdrop.style.zIndex = '1054';
        } else {
            // Andernfalls erstellen Sie ein neues Backdrop
            const backdrop = document.createElement('div');
            backdrop.className = 'modal-backdrop fade show';
            backdrop.style.zIndex = '1054';
            document.body.appendChild(backdrop);
        }
    });
});

// Funktion, die aufgerufen wird, wenn das Modal 'modal-browsed-nodes' geöffnet wird
function setupBrowsedNodesModal() {
    const browsedNodesModalElement = document.getElementById('modal-browsed-nodes');

    // Überprüfen, ob das Modal existiert
    if (!browsedNodesModalElement) {
        console.error("Element mit ID 'modal-browsed-nodes' nicht gefunden.");
        return;
    }

    // Event Listener für das 'shown.bs.modal' Event hinzufügen
    browsedNodesModalElement.addEventListener('shown.bs.modal', async function () {
        try {
            // Abrufen des ausgewählten Geräts aus dem 'modal-new-process' Modal
            const selectedDeviceSelect = document.getElementById('select-device');
            
            if (!selectedDeviceSelect) {
                throw new Error("Element mit ID 'select-device' nicht gefunden.");
            }

            const selectedDevice = selectedDeviceSelect.value;

            // Überprüfen, ob ein Gerät ausgewählt wurde
            if (!selectedDevice) {
                throw new Error("No device selected.");
            }

            // REST-Endpunkt definieren (passen Sie die URL entsprechend an)
            const endpointUrl = `${BASE_PATH}getNodes?device=${encodeURIComponent(selectedDevice)}`;

            // REST-Anfrage senden
            const response = await fetch(endpointUrl, {
                method: 'GET', // oder 'POST', je nach API
                headers: {
                    'Content-Type': 'application/json'
                    // Fügen Sie bei Bedarf weitere Header hinzu
                }
            });

            // Überprüfen, ob die Anfrage erfolgreich war
            if (!response.ok) {
                throw new Error(`Network response was not ok: ${response.statusText}`);
            }

            // Antwort verarbeiten (angenommen, es ist JSON)
            const data = await response.json();

            // Aktualisieren des Modals mit den abgerufenen Daten
            updateBrowsedNodesModal(data);

        } catch (error) {
            console.error('Fehler beim Abrufen der Browsed Nodes:', error);
            // Optional: Anzeigen einer Fehlermeldung im Modal
            displayErrorInModal(browsedNodesModalElement, error.message);
        }
    });
}

// Funktion zur Aktualisierung des Modals mit den abgerufenen Daten
function updateBrowsedNodesModal(data) {
    // Definiere browsedNodesModalElement innerhalb der Funktion
    const browsedNodesModalElement = document.getElementById('modal-browsed-nodes');
    const listGroup = browsedNodesModalElement.querySelector('.list-group');

    if (!listGroup) {
        console.error("Element mit Klasse 'list-group' im Modal nicht gefunden.");
        return;
    }

    // Leeren der bestehenden Liste
    listGroup.innerHTML = '';

    // Iterieren über die Daten und Hinzufügen von Listenelementen
    data.nodes.forEach(node => {
        const listItem = document.createElement('li');
        listItem.className = 'list-group-item';

        const formCheck = document.createElement('div');
        formCheck.className = 'form-check';

        const input = document.createElement('input');
        input.className = 'form-check-input';
        input.type = 'checkbox';
        input.id = `formCheck-${node.id}`;
        input.value = node.id;

        const label = document.createElement('label');
        label.className = 'form-check-label';
        label.htmlFor = `formCheck-${node.id}`;
        label.textContent = node.name;

        formCheck.appendChild(input);
        formCheck.appendChild(label);
        listItem.appendChild(formCheck);
        listGroup.appendChild(listItem);
    });
}

// Funktion zur Anzeige von Fehlermeldungen im Modal
function displayErrorInModal(modalElement, message) {
    const modalBody = modalElement.querySelector('.modal-body');

    if (!modalBody) {
        console.error("Element mit Klasse 'modal-body' im Modal nicht gefunden.");
        return;
    }

    // Anzeigen der Fehlermeldung
    modalBody.innerHTML = `
        <div class="alert alert-danger" role="alert">
            ${message}
        </div>
    `;
}

// Initialisieren der Funktion nach dem Laden der Seite
setupBrowsedNodesModal();