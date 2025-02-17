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


function hideAllConfigsRoute() {
    const restConfig = document.getElementById('rest-config');
    const fileConfig = document.getElementById('file-config');
    if (restConfig) restConfig.style.display = 'none';
    if (fileConfig) fileConfig.style.display = 'none';
}

hideAllConfigsRoute();

// Funktion zum Hinzufügen von Headern für beide Modals
function addHeaderToList(headerKeyInput, headerValueInput, headerList) {
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
    
    // Speichere Key und Value als Datenattribute
    listItem.dataset.headerKey = key;
    listItem.dataset.headerValue = value;

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
}

// Event-Listener für beide Header-Buttons
document.addEventListener('DOMContentLoaded', function() {
    // Für das Route-Modal
    const addHeaderButtonRoute = document.getElementById('add-header-button-r');
    const headerKeyInputRoute = document.getElementById('header-key-r');
    const headerValueInputRoute = document.getElementById('header-value-r');
    const headerListRoute = document.getElementById('header-list-r');

    if (addHeaderButtonRoute && headerKeyInputRoute && headerValueInputRoute && headerListRoute) {
        addHeaderButtonRoute.addEventListener('click', function() {
            addHeaderToList(headerKeyInputRoute, headerValueInputRoute, headerListRoute);
        });
    }

    // Für das Process-Modal
    const addHeaderButtonProcess = document.getElementById('add-header-button-p');
    if (addHeaderButtonProcess) {
        addHeaderButtonProcess.addEventListener('click', function() {
            const headerKeyInput = document.getElementById('header-key');
            const headerValueInput = document.getElementById('header-value');
            const headerList = document.getElementById('header-list');
            addHeaderToList(headerKeyInput, headerValueInput, headerList);
        });
    }
});