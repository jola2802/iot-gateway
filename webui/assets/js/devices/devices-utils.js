function hideAllConfigs() {
    // opcUaConfig.style.display = 'none';
    s7Config.style.display = 'none';
    mqttConfig.style.display = 'none';
}

function initializeNewDeviceModal() {
    // // Selektiere das Dropdown-Menü für den Gerätetyp
    const selectDeviceType = document.getElementById('select-device-type');

    // Funktion zum Ausblenden aller Konfigurationskarten
    function hideAllConfigs() {
        const configIds = ['opc-ua-config', 's7-config', 'mqtt-config'];
        configIds.forEach(id => {
            const config = document.getElementById(id);
            if (config) config.style.display = 'none';
        });
    }

    // Funktion zum Anzeigen der entsprechenden Konfigurationskarte basierend auf der Auswahl
    function showConfig(selectedType) {
        hideAllConfigs();
        const config = document.getElementById(`${selectedType}-config`);
        if (config) {
            config.style.display = 'block';

            // Falls OPC-UA ausgewählt wurde, initialisiere Sicherheitsrichtlinie und -modus
            if (selectedType === 'opc-ua') {
                initializeOpcUaSecuritySettings('');
            }
        }
    }

    // Initiales Ausblenden aller Konfigurationskarten
    hideAllConfigs();

    // Zeige die Konfigurationskarte basierend auf der aktuellen Auswahl
    const initialSelection = selectDeviceType.value;
    showConfig(initialSelection);

    // Entferne vorherige Event Listener, um doppelte Listener zu vermeiden
    const newDeviceModal = document.getElementById('modal-new-device');
    if (newDeviceModal) {
        selectDeviceType.removeEventListener('change', handleDeviceTypeChange);
    }

    // Event Listener für Änderungen im Dropdown-Menü
    function handleDeviceTypeChange() {
        const selectedValue = selectDeviceType.value;
        showConfig(selectedValue);
    }

    selectDeviceType.addEventListener('change', handleDeviceTypeChange);
}

// Funktion zur Initialisierung der Sicherheitsrichtlinien und -modi für OPC-UA
function initializeOpcUaSecuritySettings(prefix = '') {
    // Für das New Device Modal: 'select-authentication-settings'
    // Für das Edit Device Modal: 'select-authentication-settings-1'
    const selectAuthenticationSettings = document.getElementById('select-authentication-settings' + prefix);
    if (!selectAuthenticationSettings) {
        console.error('select-authentication-settings' + prefix + ' nicht gefunden.');
        return;
    }
    
    function handleAuthenticationSettingsChange() {
        if (selectAuthenticationSettings.value === 'user-pw') {
            showCredentials();
        } else {
            hideCredentials();
        }
    }
    
    // Binden Sie den Listener nur einmal
    if (!selectAuthenticationSettings.dataset.listenerBound) {
        selectAuthenticationSettings.addEventListener('change', handleAuthenticationSettingsChange);
        selectAuthenticationSettings.dataset.listenerBound = 'true';
    }
    
    // Lösen Sie den Change-Event aus, damit der initiale Zustand übernommen wird
    selectAuthenticationSettings.dispatchEvent(new Event('change'));
}    

function initializeEditDeviceModal(device_id) {
    // // Selektiere das Dropdown-Menü für den Gerätetyp
    const selectDeviceType = document.getElementById('select-device-type-1');

    // Funktion zum Ausblenden aller Konfigurationskarten
    function hideAllConfigs() {
        const configIds = ['opc-ua-config-1', 's7-config-1', 'mqtt-config-1'];
        configIds.forEach(id => {
            const config = document.getElementById(id);
            if (config) config.style.display = 'none';
        });
    }

    // Funktion zum Anzeigen der entsprechenden Konfigurationskarte basierend auf der Auswahl
    function showConfig(selectedType) {
        hideAllConfigs();
        const config = document.getElementById(`${selectedType}-config-1`);
        if (config) {
            config.style.display = 'block';

            // Falls OPC-UA ausgewählt wurde, initialisiere Sicherheitsrichtlinie und -modus
            if (selectedType === 'opc-ua') {
                initializeOpcUaSecuritySettings('-1');
                document.querySelectorAll('.datatype-column').forEach(col => col.classList.add('hidden'));
                document.querySelectorAll('td:nth-child(3)').forEach(cell => cell.classList.add('hidden')); // Verstecke alle Zellen in der Spalte
            } else {
                document.querySelectorAll('.datatype-column').forEach(col => col.classList.remove('hidden'));
                document.querySelectorAll('td:nth-child(3)').forEach(cell => cell.classList.remove('hidden')); // Zeige alle Zellen in der Spalte
            }
        }
    }

    // Funktion, um Gerätedetails basierend auf der device_id abzurufen
    async function fetchDeviceDetails(device_id) {
        try {
            // const response = await fetch(`${BASE_PATH}/getDevice/${device_id}`);
            const response = await fetch(`/api/getDevice/${device_id}`);
            if (!response.ok) {
                throw new Error(`Failed to fetch device details: ${response.status}`);
            }
            const deviceDataArr = await response.json();

            const deviceData = deviceDataArr.device; 

            console.log('Device Data:', deviceData);

            // Gerätedaten in das Modal einfügen
            document.getElementById('device-name-1').value = deviceData.deviceName || '';
            selectDeviceType.value = deviceData.deviceType || 'opc-ua';
            selectDeviceType.disabled = true;
            //disable name input field
            document.getElementById('device-name-1').disabled = true;
            showConfig(deviceData.deviceType);

            if (deviceData.deviceType === 'opc-ua') {
                // Clear old values
                document.getElementById('address-1').value = '';
                document.getElementById('select-security-policy-1').value = 'none';
                document.getElementById('select-authentication-settings-1').value = 'anonymous';
                document.getElementById('select-security-mode-1').value = 'None';

                // Fill new values
                document.getElementById('address-1').value = deviceData.address || '';
                document.getElementById('select-security-policy-1').value = deviceData.securityPolicy.String || 'none';
                document.getElementById('select-authentication-settings-1').value = deviceData.authentication || 'anonymous';
                document.getElementById('select-security-mode-1').value = deviceData.securityMode.String || 'None';

                createCredentialsFields();
                const usernameGroup = document.getElementById('username-group');
                const passwordGroup = document.getElementById('password-group');
                // username und password darf kein sql nullstring sein
                if (deviceData.username.String !== '' && deviceData.password.String !== '') {
                    // Create Username- and password-groups
                    if (usernameGroup && passwordGroup) {
                        usernameGroup.style.display = 'block';
                        passwordGroup.style.display = 'block';
                        document.getElementById('select-authentication-settings-1').value = 'user-pw';

                        const usernameInput = document.getElementById('username');
                        const passwordInput = document.getElementById('password');

                        if (usernameInput && passwordInput) {
                            usernameInput.value = deviceData.username.String || '';
                            passwordInput.value = deviceData.password.String || '';
                        } else {
                            console.error('Username or Password input not found in DOM');
                        }
                    } else {
                        document.getElementById('select-authentication-settings-1').value = 'anonymous';
                        usernameGroup.style.display = 'none';
                        passwordGroup.style.display = 'none';
                        console.error('Username group or Password group not found in DOM');
                    }
                }
                else {
                    // deleteCredentialsFields();
                    usernameGroup.style.display = 'none';
                    passwordGroup.style.display = 'none';
                    document.getElementById('select-authentication-settings-1').value = 'anonymous';
                }
                document.getElementById('acquisition-time-opc-ua-1').value = deviceData.acquisitionTime;
                
            } else if (deviceData.deviceType === 's7') {
                document.querySelector('#s7-config-1 [placeholder="192.168.2.100:102"]').value = deviceData.address || '';
                document.querySelector('#s7-config-1 [placeholder="0"]').value = deviceData.rack.String || '';
                document.querySelector('#s7-config-1 [placeholder="1"]').value = deviceData.slot.String || '';
                document.getElementById('acquisition-time-2').value = deviceData.acquisitionTime;
            } else if (deviceData.deviceType === 'mqtt') {
                document.querySelector('#mqtt-config-1 [placeholder="Type in password"]').value = deviceData.password.String || '';
            }

            // Tabelle der Datenpunkte leeren
            const datapointsTableBody = document.querySelector('#ipi-table tbody');
            datapointsTableBody.innerHTML = '';

            if (deviceData.datapoint) {
                // Tabelle für Datenpunkte
                const datapointsTableBody = document.querySelector('#ipi-table tbody');
                datapointsTableBody.innerHTML = '';

                deviceData.datapoint.forEach(datapoint => {
                    const row = document.createElement('tr');
                    row.setAttribute('datapoint-id', datapoint.datapointId);

                    const idCell = document.createElement('td');
                    idCell.textContent = datapoint.datapointId;
                    idCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(idCell);

                    const nameCell = document.createElement('td');
                    nameCell.textContent = datapoint.name;
                    nameCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(nameCell);

                    const datatypeCell = document.createElement('td');
                    datatypeCell.textContent = datapoint.datatype || 'INT';
                    row.appendChild(datatypeCell);

                    const addressCell = document.createElement('td');
                    addressCell.textContent = datapoint.address || '';
                    addressCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(addressCell);

                    const actionCell = document.createElement('td');
                    actionCell.innerHTML = `
                    <a href="#" class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" 
                        style="margin-left: 5px;" 
                        onclick="confirmDeleteDatapoint('${datapoint.datapointId}', event)">
                        <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                    </a>
                    `;
                    row.appendChild(actionCell);

                    datapointsTableBody.appendChild(row);
                });

            }
            datapointsTableBody.appendChild(createEmptyRow(deviceData.deviceType));

        } catch (error) {
            console.error(`Error fetching device details: ${error.message}`);
        }
    }

    // Funktion zum Hinzufügen der Username- und Password-Felder
    function createCredentialsFields(prefix = '') {
        // Verwenden Sie den Container, in den die Felder eingefügt werden sollen.
        // Für das New Device Modal muss im HTML <div id="opc-ua-credentials"></div> vorhanden sein.
        // Für das Edit Device Modal muss im HTML <div id="opc-ua-credentials-1"></div> vorhanden sein.
        const container = document.getElementById('opc-ua-credentials' + prefix);
        if (!container) {
          console.error('Credentials container not found: opc-ua-credentials' + prefix);
          return;
        }
        
        // Falls noch nicht vorhanden, erstellen Sie die Username-Feldgruppe.
        if (!document.getElementById('username-group' + prefix)) {
          const usernameGroup = document.createElement('div');
          usernameGroup.id = 'username-group' + prefix;
          usernameGroup.className = 'form-group mb-3';
          usernameGroup.innerHTML = `
            <label class="form-label" for="username${prefix}"><strong>Username</strong></label>
            <input type="text" class="form-control" id="username${prefix}" placeholder="Enter username">
          `;
          container.appendChild(usernameGroup);
        }
        
        // Falls noch nicht vorhanden, erstellen Sie die Password-Feldgruppe.
        if (!document.getElementById('password-group' + prefix)) {
          const passwordGroup = document.createElement('div');
          passwordGroup.id = 'password-group' + prefix;
          passwordGroup.className = 'form-group mb-3';
          passwordGroup.innerHTML = `
            <label class="form-label" for="password${prefix}"><strong>Password</strong></label>
            <input type="password" class="form-control" id="password${prefix}" placeholder="Enter password">
          `;
          container.appendChild(passwordGroup);
        }
      }
      
    
    // Initiales Ausblenden aller Konfigurationskarten
    hideAllConfigs();

    // Zeige die Konfigurationskarte basierend auf der aktuellen Auswahl
    const initialSelection = selectDeviceType.value;
    showConfig(initialSelection);

    // Gerätedetails abrufen
    fetchDeviceDetails(device_id);

    // Entferne vorherige Event Listener, um doppelte Listener zu vermeiden
    const newEditDeviceModal = document.getElementById('modal-edit-device');
    if (newEditDeviceModal) {
        selectDeviceType.removeEventListener('change', handleDeviceTypeChange);
    }

    // Event Listener für Änderungen im Dropdown-Menü
    function handleDeviceTypeChange() {
        const selectedValue = selectDeviceType.value;
        showConfig(selectedValue);
    }

    selectDeviceType.addEventListener('change', handleDeviceTypeChange);

    // Funktion zur Initialisierung der Sicherheitsrichtlinien und -modi für OPC-UA
    function initializeOpcUaSecuritySettings(prefix = '') {
        // Für New Device: id="select-authentication-settings"
        // Für Edit Device: id="select-authentication-settings-1"
        const authSelect = document.getElementById('select-authentication-settings' + prefix);
        if (!authSelect) {
          console.error('Authentication select not found: select-authentication-settings' + prefix);
          return;
        }
        function handleAuthChange() {
          if (authSelect.value === 'user-pw') {
            showCredentials(prefix);
          } else {
            hideCredentials(prefix);
          }
        }
        // Binden Sie den Listener nur einmal, um Mehrfachbindungen zu vermeiden.
        if (!authSelect.dataset.listenerBound) {
          authSelect.addEventListener('change', handleAuthChange);
          authSelect.dataset.listenerBound = 'true';
        }
        // Lösen Sie den Change-Event aus, damit der aktuelle Zustand übernommen wird.
        authSelect.dispatchEvent(new Event('change'));
      }

    // Save device_id in web storage
    localStorage.setItem('device_id', device_id);
}

// Funktion zum Hinzufügen der Username- und Password-Felder
function showCredentials(prefix = '') {
    // Der Container, in den die dynamischen Felder eingefügt werden sollen.
    // Für New Device: id="opc-ua-credentials"
    // Für Edit Device: id="opc-ua-credentials-1"
    const container = document.getElementById('opc-ua-credentials' + prefix);
    if (!container) {
      console.error('Credentials container not found: opc-ua-credentials' + prefix);
      return;
    }
    // Falls noch nicht vorhanden, erstellen Sie die Username-Feldgruppe.
    if (!document.getElementById('username' + prefix)) {
      const usernameGroup = document.createElement('div');
      usernameGroup.id = 'username-group' + prefix;
      usernameGroup.className = 'form-group mb-3';
      usernameGroup.innerHTML = `
        <label class="form-label" for="username${prefix}"><strong>Username</strong></label>
        <input type="text" class="form-control" id="username${prefix}" placeholder="Enter username">
      `;
      container.appendChild(usernameGroup);
    }
    // Falls noch nicht vorhanden, erstellen Sie die Password-Feldgruppe.
    if (!document.getElementById('password' + prefix)) {
      const passwordGroup = document.createElement('div');
      passwordGroup.id = 'password-group' + prefix;
      passwordGroup.className = 'form-group mb-3';
      passwordGroup.innerHTML = `
        <label class="form-label" for="password${prefix}"><strong>Password</strong></label>
        <input type="password" class="form-control" id="password${prefix}" placeholder="Enter password">
      `;
      container.appendChild(passwordGroup);
    }
  }
  
function hideCredentials(prefix = '') {
const usernameGroup = document.getElementById('username-group' + prefix);
const passwordGroup = document.getElementById('password-group' + prefix);
if (usernameGroup) usernameGroup.remove();
if (passwordGroup) passwordGroup.remove();
}
  

function createEmptyRow(deviceType) {
    const emptyRow = document.createElement('tr');

    // Spalte: Datapoint ID
    const idCell = document.createElement('td');
    const idInput = document.createElement('input');
    idInput.type = 'text';
    idInput.className = 'form-control';
    idInput.placeholder = 'Enter ID';
    idCell.appendChild(idInput);
    emptyRow.appendChild(idCell);

    // Spalte: Datapoint Name
    const nameCell = document.createElement('td');
    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.className = 'form-control';
    nameInput.placeholder = 'Enter Name';
    nameCell.appendChild(nameInput);
    emptyRow.appendChild(nameCell);

    // Spalte: Datatype – nur bei s7 als Dropdown; bei opc-ua statisch oder leer
    const datatypeCell = document.createElement('td');
    if (deviceType === 's7') {
        const datatypeSelect = document.createElement('select');
        datatypeSelect.className = 'form-select';
        ['INT', 'REAL', 'BOOL', 'STRING'].forEach(type => {
            const option = document.createElement('option');
            option.value = type;
            option.textContent = type;
            datatypeSelect.appendChild(option);
        });
        datatypeCell.appendChild(datatypeSelect);
    } else if (deviceType === 'opc-ua') {
        // Bei OPC-UA ist die Auswahl des Datentyps nicht relevant.
        // Wir können hier einen statischen Text anzeigen oder das Feld leer lassen.
        datatypeCell.textContent = 'N/A';
    } else {
        // Fallback: Dropdown erstellen
        const datatypeSelect = document.createElement('select');
        datatypeSelect.className = 'form-select';
        ['INT', 'REAL', 'BOOL', 'STRING'].forEach(type => {
            const option = document.createElement('option');
            option.value = type;
            option.textContent = type;
            datatypeSelect.appendChild(option);
        });
        datatypeCell.appendChild(datatypeSelect);
    }
    emptyRow.appendChild(datatypeCell);

    // Spalte: Address / Node ID
    const addressCell = document.createElement('td');
    const addressInput = document.createElement('input');
    addressInput.type = 'text';
    addressInput.className = 'form-control';
    addressInput.placeholder = 'Enter Address / Node ID';
    addressCell.appendChild(addressInput);
    emptyRow.appendChild(addressCell);

    // Spalte: Aktion (Save-Button)
    const actionCell = document.createElement('td');
    const saveButton = document.createElement('button');
    saveButton.type = 'button';
    saveButton.className = 'btn btn-success';
    saveButton.textContent = 'Save';
    saveButton.addEventListener('click', () => {
        // Bei s7 wird der ausgewählte Datentyp übergeben,
        // bei opc-ua kann ein fester Wert oder null übergeben werden.
        let datatypeValue = null;
        if (deviceType === 's7') {
            // Hier nehmen wir den Wert aus dem select-Feld
            datatypeValue = emptyRow.querySelector('select').value;
        }
        saveDatapoint(idInput.value, nameInput.value, datatypeValue, addressInput.value, deviceType);
    });
    actionCell.appendChild(saveButton);
    emptyRow.appendChild(actionCell);

    return emptyRow;
}

function saveDatapoint(id, name, datatype, address, deviceType) {
    if (!name || !address || !datatype && deviceType === 's7') {
        alert('Please fill all fields before saving!');
        return;
    }

    const datapointId = id ? id : Math.floor(Math.random() * 1000);

    const newRow = document.createElement('tr');


    // use datapointId when id is empty
    if (!id) {
        id = datapointId;
    }

    const idCell = document.createElement('td');
    idCell.textContent = id;
    idCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(idCell);

    const nameCell = document.createElement('td');
    nameCell.textContent = name;
    nameCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(nameCell);

    const datatypeCell = document.createElement('td');
    datatypeCell.textContent = datatype;
    datatypeCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(datatypeCell);

    const addressCell = document.createElement('td');
    addressCell.textContent = address;
    addressCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(addressCell);

    const actionCell = document.createElement('td');
    actionCell.innerHTML = `
      <a href="#" class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" 
         style="margin-left: 5px;" 
         onclick="confirmDeleteDatapoint('${datapointId}', event)">
          <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
      </a>
    `;
    newRow.appendChild(actionCell);

    const tableBody = document.querySelector('#ipi-table tbody');
    const lastRow = tableBody.lastChild;
    tableBody.insertBefore(newRow, lastRow);

    const inputs = lastRow.querySelectorAll('input, select');
    inputs.forEach(input => (input.value = ''));
}

function confirmDeleteDatapoint(datapointId, event) {
    // Standardverhalten verhindern (z. B. Absenden eines Formulars)
    if (event) {
        event.preventDefault();
    }
    
    if (confirm(`Möchten Sie den Datapoint mit der ID ${datapointId} wirklich löschen?`)) {
        // Entfernen Sie nur die entsprechende Zeile aus der Tabelle.
        const row = document.querySelector(`tr[datapoint-id="${datapointId}"]`); 
        if (row) {
            row.remove();
        }
    }
}

// Ergänze diese Hilfsfunktion irgendwo im selben Skript (z. B. unterhalb von fetchAndPopulateDevices)
function confirmDeleteDevice(deviceId) {
    // Einfaches Bestätigungs-Dialogfenster
    if (confirm(`Möchten Sie das Gerät mit der ID ${deviceId} wirklich löschen?`)) {
        // API-Aufruf an das Backend zum Löschen (per DELETE oder POST, je nach Server-Implementierung)
        fetch(`/api/delete-device/${deviceId}`, { method: 'DELETE' })
            .then(response => {
                if (!response.ok) {
                    throw new Error(`Fehler beim Löschen (HTTP ${response.status}).`);
                }
                alert('Gerät erfolgreich gelöscht!');
                // Seite aktualisieren oder Tabelle neu laden, damit das gelöschte Gerät nicht mehr angezeigt wird
                // fetchAndPopulateDevices();
                location.reload();
            })
            .catch(error => {
                console.error('Fehler beim Löschen des Geräts:', error);
                alert('Ein unbekannter Fehler ist aufgetreten.');
            });
    }
}