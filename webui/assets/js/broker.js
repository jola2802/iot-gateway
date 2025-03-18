async function fetchAndPopulateBrokerUsers() {
    try {
        const response = await fetch('/api/getBrokerUsers');

        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }
        
        // JSON-Daten lesen
        const users = await response.json();

        // Tabelle referenzieren
        const tableBody = document.querySelector('#table-user-broker tbody');

        // Spinner-Zeile entfernen
        tableBody.innerHTML = '';

        // Benutzer einfügen
        users.forEach(user => {
            const row = document.createElement('tr');

            // Username
            const usernameCell = document.createElement('td');
            usernameCell.textContent = user.username;
            row.appendChild(usernameCell);

            // Password (versteckt oder dargestellt)
            const passwordCell = document.createElement('td');
            passwordCell.className = 'text-center align-middle';
            const inputGroup = document.createElement('div');
            inputGroup.className = 'input-group';
            inputGroup.style.maxWidth = '250px';

            const passwordInput = document.createElement('input');
            passwordInput.className = 'form-control passInput';
            passwordInput.type = 'password';
            passwordInput.value = user.password;
            passwordInput.style.maxWidth = '200px';
            inputGroup.appendChild(passwordInput);

            const toggleButton = document.createElement('button');
            toggleButton.className = 'btn btn-light togglePassword';
            toggleButton.type = 'button';
            toggleButton.setAttribute('title', 'Show/Hide Password');
            toggleButton.innerHTML = '<i class="fas fa-eye-slash mx-2"></i>';
            toggleButton.addEventListener('click', () => {
                if (passwordInput.type === 'password') {
                    passwordInput.type = 'text';
                    toggleButton.innerHTML = '<i class="fas fa-eye mx-2"></i>';
                } else {
                    passwordInput.type = 'password';
                    toggleButton.innerHTML = '<i class="fas fa-eye-slash mx-2"></i>';
                }
            });            
            inputGroup.appendChild(toggleButton);

            passwordCell.appendChild(inputGroup);
            row.appendChild(passwordCell);

            // ACL Topic und Permissions
            const aclCell = document.createElement('td');
            aclCell.style.whiteSpace = 'pre-line'; // Zeilenumbruch innerhalb der Zelle
            if (user.aclEntries && Array.isArray(user.aclEntries)) {
                aclCell.innerHTML = user.aclEntries.map(entry => {
                    // Mapping von Permission-Zahlen zu Klartext
                    const permissionText = {
                        3: "Read and Write",
                        2: "Write only",
                        1: "Read only",
                        0: "No Access"
                    }[entry.permission] || "Unknown";

                    // Topic und Permission in eine Zeile packen
                    return `${entry.topic} (${permissionText})`;
                }).join('\n'); // Zeilenumbruch zwischen den Einträgen innerhalb derselben Zelle
            } else {
                aclCell.textContent = "No ACL entries found";
            }
            row.appendChild(aclCell);

            // Actions
            const actionsCell = document.createElement('td');
            actionsCell.className = 'text-center align-middle';
            actionsCell.style.height = '60px';

            const editButton = document.createElement('a');
            editButton.className = 'btn btnMaterial btn-flat success semicircle';
            editButton.href = '#';
            editButton.innerHTML = '<i class="fas fa-pen"></i>';
            editButton.addEventListener('click', () => {
                editBrokerUser(user.username);
            });
            actionsCell.appendChild(editButton);

            const deleteButton = document.createElement('a');
            deleteButton.className = 'btn btnMaterial btn-flat accent btnNoBorders checkboxHover';
            deleteButton.style.marginLeft = '5px';
            deleteButton.setAttribute('data-bs-toggle', 'modal');
            deleteButton.setAttribute('data-bs-target', '#delete-modal');
            deleteButton.innerHTML = '<i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>';
            deleteButton.addEventListener('click', () => {
                deleteBrokerUser(user.username);
            });
            actionsCell.appendChild(deleteButton);

            row.appendChild(actionsCell);

            // Zeile in die Tabelle einfügen
            tableBody.appendChild(row);
        });
    } catch (error) {
        console.error('Error fetching broker users:', error);
    }
}

async function deleteBrokerUser(username) {
    const response = await fetch(`/api/delete-broker-user/${username}`, {
        method: 'DELETE'
    });
    
    if (!response.ok) {
        throw new Error(`HTTP error! Status: ${response.status}`);
    }

    // Tabelle neu laden
    fetchAndPopulateBrokerUsers();

    alert("User deleted successfully");
}

// Aufruf der Funktion, um die Tabelle zu befüllen
fetchAndPopulateBrokerUsers();

// Event-Listener für den Add-Button
document.querySelector('.btn-primary[type="button"]').addEventListener('click', () => {
    const selectPermissionLevel = document.getElementById('select-permission-level');
    const mqttTopicInput = document.getElementById('mqtt-topic');
    const selectedPermission = selectPermissionLevel.value;
    const permissionText = selectPermissionLevel.options[selectPermissionLevel.selectedIndex].text;
    const topicList = document.getElementById('list-permission-topics');

    // Eingabewerte holen und prüfen
    const topic = mqttTopicInput.value.trim();

    if (!topic) {
        alert('Please enter a MQTT topic.');
        return;
    }

    // Neues Listenelement erstellen
    const listItem = document.createElement('li');
    listItem.className = 'list-group-item d-flex justify-content-between align-items-center';
    listItem.setAttribute('data-topic', topic);
    listItem.setAttribute('data-permission', selectedPermission);

    listItem.innerHTML = `
        <span><strong>${topic}</strong>: ${permissionText}</span>
        <button class="btn btn-danger btn-sm" type="button">Remove</button>
    `;

    // Entfernen-Button hinzufügen
    const removeButton = listItem.querySelector('button');
    removeButton.addEventListener('click', () => {
        topicList.removeChild(listItem);
    });

    // Neues Element zur Liste hinzufügen
    topicList.appendChild(listItem);

    // Eingabefelder leeren
    mqttTopicInput.value = '';
    selectPermissionLevel.selectedIndex = 0;
});

/**
 * Funktion zum Speichern eines neuen Broker-Benutzers.
 * Diese Funktion sammelt die Benutzerdaten und ACL-Einträge,
 * validiert die Eingaben, sendet die Daten an den REST-Endpunkt
 * und behandelt die Antwort.
 */
document.getElementById("btn-save-new-user").addEventListener('click', () => {
    // Werte aus den Eingabefeldern abrufen
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value.trim();

    // Validierung der Eingabefelder
    if (username === '' || password === '') {
        alert('Please enter both username and password.');
        return;
    }

    // ACL-Einträge sammeln
    const aclListItems = document.querySelectorAll('#list-permission-topics li');
    const acls = [];

    aclListItems.forEach(item => {
        const topic = item.getAttribute('data-topic');
        // Permission als Integer parsen
        const permission = parseInt(item.getAttribute('data-permission'), 10);
        acls.push({ topic, permission });
    });

    // Validierung der ACL-Einträge
    if (acls.length === 0) {
        alert('Please add at least one ACL permission.');
        return;
    }

    // Daten für den Versand vorbereiten
    const userData = {
        username,
        password,
        acls
    };

    // POST-Anfrage mit fetch an den REST-Endpunkt senden
    fetch(`/api/add-broker-user`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(userData)
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(err => Promise.reject(err));
        }
        return response.json();
    })
    .then(() => {
        // Eingabefelder und ACL-Liste leeren
        document.getElementById('username').value = '';
        document.getElementById('password').value = '';
        document.getElementById('list-permission-topics').innerHTML = '';

        // Modal schließen
        const modalElement = document.getElementById('modal-new-user');
        const modal = bootstrap.Modal.getInstance(modalElement);
        if (modal) {
            modal.hide();
        }

        // Benutzer über den Erfolg informieren
        alert('User added successfully!');
        
        // Tabelle neu laden
        fetchAndPopulateBrokerUsers();
    })
    .catch(error => {
        console.error('Error adding user:', error);
        alert(`User could not be added. Error: ${error.message || 'Unknown error'}`);
    });
});

async function editBrokerUser(username) {
    try {
        // Daten des Benutzers von der API abrufen
        const response = await fetch(`/api/getBrokerUser/${username}`);
        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }
        const userData = await response.json();

        // Modal-Felder befüllen
        document.getElementById('username').value = userData.username;
        document.getElementById('username').disabled = true; // Username darf nicht bearbeitet werden
        document.getElementById('password').value = userData.password;

        // ACL-Liste befüllen
        const topicList = document.getElementById('list-permission-topics');
        topicList.innerHTML = ''; // Vorherige Einträge löschen
        userData.aclEntries.forEach(aclEntry => {
            const permissionText = {
                3: "Read and Write",
                2: "Write only",
                1: "Read only",
                0: "No Access"
            }[aclEntry.permission] || "Unknown";

            const listItem = document.createElement('li');
            listItem.className = 'list-group-item d-flex justify-content-between align-items-center';
            listItem.setAttribute('data-topic', aclEntry.topic);
            listItem.setAttribute('data-permission', aclEntry.permission);

            listItem.innerHTML = `
                <span><strong>${aclEntry.topic}</strong>: ${permissionText}</span>
                <button class="btn btn-danger btn-sm" type="button">Remove</button>
            `;

            // Entfernen-Button hinzufügen
            const removeButton = listItem.querySelector('button');
            removeButton.addEventListener('click', () => {
                topicList.removeChild(listItem);
            });

            topicList.appendChild(listItem);
        });

        // Modal-Titel anpassen
        const modalTitle = document.querySelector('#modal-new-user .modal-title');
        modalTitle.textContent = 'Edit User';

        // Toggle-Button für Passwort hinzufügen
        const passwordInput = document.getElementById('password');
        passwordInput.type = 'text'; // Passwort anzeigen

        // Save-Button-Handler aktualisieren
        const saveButton = document.getElementById('btn-save-new-user');

        // Modal öffnen
        const modalElement = document.getElementById('modal-new-user');
        const modal = new bootstrap.Modal(modalElement);
        modal.show();
    } catch (error) {
        console.error('Error loading user data:', error);
        alert('Error loading user data. Please try again.');
    }
}

async function connectToBrokerAndDisplayTopics() {
    try {
        // Anmeldedaten von der REST-API abrufen
        const response = await fetch(`/api/getBrokerLogin`);
        if (!response.ok) {
            throw new Error(`HTTP error! Status: ${response.status}`);
        }

        const loginData = await response.json();
        const { username, password, brokerUrl } = loginData;

        if (!username || !password || !brokerUrl) {
            throw new Error('Missing login data.');
        }

        const options = {
            username: username,
            password: password,
            clean: true,
            connectTimeout: 3000,
            clientId: 'mqttjs_' + Math.random().toString(16).substr(2, 8),
        }

        // Verbindung zum MQTT-Broker herstellen
        const client = mqtt.connect(brokerUrl, options);

        // Wurzel des Baums
        const treeRoot = {};

        // Nachricht empfangen und Baum aktualisieren
        client.on('message', (topic, message) => {
            const topicParts = topic.split('/');
            let currentNode = treeRoot;
        
            // Rekursive Struktur erzeugen
            topicParts.forEach((part, index) => {
                if (!currentNode[part]) {
                    currentNode[part] = {}; // Neuen Node erstellen, falls nicht vorhanden
                }
                if (index === topicParts.length - 1) {
                    // Den Wert im letzten Node speichern
                    currentNode[part]._value = message.toString();
                }
                currentNode = currentNode[part];
            });
        
            // Baum neu rendern
            renderTree(treeRoot, document.getElementById('mqtt-topic-tree'));
        });

        
        client.on('connect', () => {
            // console.log('Verbindung hergestellt');
            const topics = ['$SYS/broker/#', 'data/#', 'iot-gateway/#'];
            client.subscribe(topics, (err) => {
            if (err) {
                console.error('Error subscribing to topics:', err);
            } else {
                // console.log('Erfolgreich alle Topics abonniert.');
            }
            });
        });

        client.on('error', (error) => {
            console.error('Error connecting:', error);
        });

        client.on('close', () => {
            // console.log('Verbindung geschlossen.');
        });
    } catch (error) {
        console.error('Error:', error);
    }
}

// Baum rendern
function renderTree(tree, container) {
    container.innerHTML = ''; // Vorherige Inhalte löschen

    Object.keys(tree).forEach((key) => {
        if (key === '_value') return; // "_value" ist kein sichtbarer Key

        const listItem = document.createElement('li');
        listItem.textContent = key;

        // Wert hinzufügen, falls vorhanden
        if (tree[key]._value !== undefined) {
            const valueSpan = document.createElement('span');
            valueSpan.style.marginLeft = '10px';
            valueSpan.style.color = 'gray';
            valueSpan.textContent = `: ${tree[key]._value}`;
            listItem.appendChild(valueSpan);
        }

        // Rekursion für verschachtelte Objekte
        const childKeys = Object.keys(tree[key]).filter(k => k !== '_value');
        if (childKeys.length > 0) {
            const nestedList = document.createElement('ul');
            renderTree(tree[key], nestedList);
            listItem.appendChild(nestedList);
        }

        container.appendChild(listItem);
    });
}

// Verbindung herstellen und Topics anzeigen
// connectToBrokerAndDisplayTopics();

// Funktion zum Zurücksetzen des Modals
function resetModal() {
    // Alle Eingabefelder leeren
    document.getElementById('username').value = '';
    document.getElementById('username').disabled = false; // Username wieder editierbar machen
    document.getElementById('password').value = '';
    document.getElementById('list-permission-topics').innerHTML = ''; // ACL-Liste leeren

    // Modal-Titel zurücksetzen
    const modalTitle = document.querySelector('#modal-new-user .modal-title');
    modalTitle.textContent = 'Add New User';

    // Toggle-Button für Passwort auf den Standard zurücksetzen (falls sichtbar)
    const passwordInput = document.getElementById('password');
    passwordInput.type = 'password'; // Passwortfeld wieder verstecken
}

// Event Listener für das Schließen des Modals hinzufügen
document.getElementById('modal-new-user').addEventListener('hidden.bs.modal', () => {
    resetModal();
});
