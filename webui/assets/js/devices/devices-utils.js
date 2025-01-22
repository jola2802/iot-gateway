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
                initializeOpcUaSecuritySettings();
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

    // Funktion zur Initialisierung der Sicherheitsrichtlinien und -modi für OPC-UA
    function initializeOpcUaSecuritySettings() {
        const opcUaConfig = document.getElementById('opc-ua-config');
        if (!opcUaConfig) {
            console.error('Element mit der ID "opc-ua-config" wurde nicht gefunden.');
            return;
        }

        const selectSecurityPolicy = document.getElementById('select-security-policy');
        const selectAuthenticationSettings = document.getElementById('select-authentication-settings');
        const selectSecurityMode = document.getElementById('select-security-mode');

        // Funktion zum Hinzufügen der Username- und Password-Felder
        function showCredentials() {
            // Erste Spalte (links)
            const firstCol = opcUaConfig.querySelectorAll('.row .col')[0];
            if (!firstCol) {
                console.error('Erste Spalte nicht gefunden.');
                return;
            }

            // Zweite Spalte (rechts)
            const secondCol = opcUaConfig.querySelectorAll('.row .col')[1];
            if (!secondCol) {
                console.error('Zweite Spalte nicht gefunden.');
                return;
            }

            // Prüfe, ob das Username-Feld bereits existiert
            const existingUsernameGroup = firstCol.querySelector('#username-group');
            if (!existingUsernameGroup) {
                // Erstelle die Username-Feldgruppe
                const usernameGroup = document.createElement('div');
                usernameGroup.className = 'form-group mb-3';
                usernameGroup.id = 'username-group';

                const usernameLabel = document.createElement('label');
                usernameLabel.className = 'form-label';
                usernameLabel.setAttribute('for', 'username');
                usernameLabel.innerHTML = '<strong>Username</strong>';

                const usernameInput = document.createElement('input');
                usernameInput.className = 'form-control';
                usernameInput.type = 'text';
                usernameInput.id = 'username';
                usernameInput.placeholder = 'Enter username here';

                usernameGroup.appendChild(usernameLabel);
                usernameGroup.appendChild(usernameInput);

                // Füge das Username-Feld nach dem Address-Feld ein
                const addressGroup = firstCol.querySelector('.form-group');
                if (addressGroup) {
                    addressGroup.parentNode.insertBefore(usernameGroup, addressGroup.nextSibling);
                } else {
                    // Falls das Address-Feld nicht gefunden wird, füge es am Ende ein
                    firstCol.appendChild(usernameGroup);
                }
            }

            // Prüfe, ob das Password-Feld bereits existiert
            const existingPasswordGroup = secondCol.querySelector('#password-group');
            if (!existingPasswordGroup) {
                // Erstelle die Password-Feldgruppe
                const passwordGroup = document.createElement('div');
                passwordGroup.className = 'form-group mb-3';
                passwordGroup.id = 'password-group';

                const passwordLabel = document.createElement('label');
                passwordLabel.className = 'form-label';
                passwordLabel.setAttribute('for', 'password');
                passwordLabel.innerHTML = '<strong>Password</strong>';

                const passwordInput = document.createElement('input');
                passwordInput.className = 'form-control';
                passwordInput.type = 'password';
                passwordInput.id = 'password';
                passwordInput.placeholder = 'Enter password';

                passwordGroup.appendChild(passwordLabel);
                passwordGroup.appendChild(passwordInput);

                // Füge das Password-Feld nach dem Authentication Settings-Feld ein
                const authSettingsGroup = secondCol.querySelector('#select-authentication-settings').parentElement;
                if (authSettingsGroup) {
                    console.log("inserBefore");
                    authSettingsGroup.parentNode.insertBefore(passwordGroup, authSettingsGroup.nextSibling);
                } else {
                    console.log("appendChild");
                    // Falls das Authentication Settings-Feld nicht gefunden wird, füge es am Ende ein
                    secondCol.appendChild(passwordGroup);
                }

                // Füge das Username-Feld nach dem Address-Feld ein
                // const addressGroup = firstCol.querySelector('.form-group');
                // if (addressGroup) {
                //     addressGroup.parentNode.insertBefore(usernameGroup, addressGroup.nextSibling);
                // } else {
                //     // Falls das Address-Feld nicht gefunden wird, füge es am Ende ein
                //     firstCol.appendChild(usernameGroup);
                // }
            }
        }

        // Funktion zum Entfernen der Username- und Password-Felder
        function hideCredentials() {
            // Entferne das Username-Feld
            const usernameGroup = opcUaConfig.querySelector('#username-group');
            if (usernameGroup) {
                usernameGroup.remove();
            }

            // Entferne das Password-Feld
            const passwordGroup = opcUaConfig.querySelector('#password-group');
            if (passwordGroup) {
                passwordGroup.remove();
            }
        }

        // Funktion zur Behandlung der Änderung der Authentifizierungseinstellungen
        function handleAuthenticationSettingsChange() {
            const selectedAuth = selectAuthenticationSettings.value;
            // console.log(`Authentication Settings changed to: ${selectedAuth}`);
            if (selectedAuth === 'user-pw') {
                showCredentials();
            } else {
                hideCredentials();
            }
        }

        // Initiale Behandlung basierend auf der aktuellen Auswahl
        handleAuthenticationSettingsChange();

        // Entferne vorherige Event Listener, um doppelte Listener zu vermeiden
        selectAuthenticationSettings.removeEventListener('change', handleAuthenticationSettingsChange);

        // Füge Event Listener hinzu
        selectAuthenticationSettings.addEventListener('change', handleAuthenticationSettingsChange);
    }
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
                initializeOpcUaSecuritySettings();
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
            //Disbale the device type dropdown
            selectDeviceType.disabled = true;
            showConfig(deviceData.deviceType);

            if (deviceData.deviceType === 'opc-ua') {
                document.getElementById('address-1').value = deviceData.address || '';
                document.getElementById('select-security-policy-1').value = deviceData.securityPolicy.String || 'none';
                document.getElementById('select-authentication-settings-1').value = deviceData.authentication || 'anonymous';
                document.getElementById('select-security-mode-1').value = deviceData.securityMode.String || 'None';

                if (deviceData.authentication === 'user-pw') {
                    // Create Username- and password-groups
                    createCredentialsFields();

                    const usernameGroup = document.getElementById('username-group');
                    const passwordGroup = document.getElementById('password-group');

                    if (usernameGroup && passwordGroup) {
                        usernameGroup.style.display = 'block';
                        passwordGroup.style.display = 'block';

                        const usernameInput = document.getElementById('username');
                        const passwordInput = document.getElementById('password');

                        if (usernameInput && passwordInput) {
                            usernameInput.value = deviceData.username.String || '';
                            passwordInput.value = deviceData.password.String || '';
                        } else {
                            console.error('Username or Password input not found in DOM');
                        }
                    } else {
                        console.error('Username group or Password group not found in DOM');
                    }
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

            if (deviceData.datapoint) {
                // Tabelle für Datenpunkte
                const datapointsTableBody = document.querySelector('#ipi-table tbody');
                datapointsTableBody.innerHTML = '';

                deviceData.datapoint.forEach(datapoint => {
                    const row = document.createElement('tr');

                    const idCell = document.createElement('td');
                    idCell.textContent = datapoint.datapointId;
                    idCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(idCell);

                    const nameCell = document.createElement('td');
                    nameCell.textContent = datapoint.name;
                    nameCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(nameCell);

                    const addressCell = document.createElement('td');
                    addressCell.textContent = datapoint.address || '';
                    addressCell.style.color = 'rgb(121, 121, 121)';
                    row.appendChild(addressCell);

                    const actionCell = document.createElement('td');
                    actionCell.innerHTML = `
                        <a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" style="margin-left: 5px;" data-bs-toggle="modal" data-bs-target="#delete-modal">
                            <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                        </a>
                    `;
                    row.appendChild(actionCell);

                    datapointsTableBody.appendChild(row);
                });

                datapointsTableBody.appendChild(createEmptyRow());
            }

        } catch (error) {
            console.error(`Error fetching device details: ${error.message}`);
        }
    }

    // Funktion zum Hinzufügen der Username- und Password-Felder
    function createCredentialsFields() {
        const opcUaConfig = document.getElementById('opc-ua-config-1');
        if (!opcUaConfig) {
            console.error('OPC-UA Config not found in DOM');
            return;
        }
    
        // Erste Spalte (links)
        const firstCol = opcUaConfig.querySelectorAll('.row .col')[0];
        if (firstCol && !document.getElementById('username-group')) {
            const usernameGroup = document.createElement('div');
            usernameGroup.className = 'form-group mb-3';
            usernameGroup.id = 'username-group';
    
            const usernameLabel = document.createElement('label');
            usernameLabel.className = 'form-label';
            usernameLabel.setAttribute('for', 'username');
            usernameLabel.innerHTML = '<strong>Username</strong>';
    
            const usernameInput = document.createElement('input');
            usernameInput.className = 'form-control';
            usernameInput.type = 'text';
            usernameInput.id = 'username';
            usernameInput.placeholder = 'Enter username';
    
            usernameGroup.appendChild(usernameLabel);
            usernameGroup.appendChild(usernameInput);
            firstCol.appendChild(usernameGroup);
        }
    
        // Zweite Spalte (rechts)
        const secondCol = opcUaConfig.querySelectorAll('.row .col')[1];
        if (secondCol && !document.getElementById('password-group')) {
            const passwordGroup = document.createElement('div');
            passwordGroup.className = 'form-group mb-3';
            passwordGroup.id = 'password-group';
    
            const passwordLabel = document.createElement('label');
            passwordLabel.className = 'form-label';
            passwordLabel.setAttribute('for', 'password');
            passwordLabel.innerHTML = '<strong>Password</strong>';
    
            const passwordInput = document.createElement('input');
            passwordInput.className = 'form-control';
            passwordInput.type = 'password';
            passwordInput.id = 'password';
            passwordInput.placeholder = 'Enter password';
    
            passwordGroup.appendChild(passwordLabel);
            passwordGroup.appendChild(passwordInput);
            secondCol.appendChild(passwordGroup);
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
    function initializeOpcUaSecuritySettings() {
        const opcUaConfig = document.getElementById('opc-ua-config-1');
        if (!opcUaConfig) {
            console.error('Element mit der ID "opc-ua-config" wurde nicht gefunden.');
            return;
        }

        const selectSecurityPolicy = document.getElementById('select-security-policy-1');
        const selectAuthenticationSettings = document.getElementById('select-authentication-settings-1');
        const selectSecurityMode = document.getElementById('select-security-mode-1');

        // Funktion zum Hinzufügen der Username- und Password-Felder
        function showCredentials() {
            // Erste Spalte (links)
            const firstCol = opcUaConfig.querySelectorAll('.row .col')[0];
            if (!firstCol) {
                console.error('Erste Spalte nicht gefunden.');
                return;
            }

            // Zweite Spalte (rechts)
            const secondCol = opcUaConfig.querySelectorAll('.row .col')[1];
            if (!secondCol) {
                console.error('Zweite Spalte nicht gefunden.');
                return;
            }

            // Prüfe, ob das Username-Feld bereits existiert
            const existingUsernameGroup = firstCol.querySelector('#username-group');
            if (!existingUsernameGroup) {
                // Erstelle die Username-Feldgruppe
                const usernameGroup = document.createElement('div');
                usernameGroup.className = 'form-group mb-3';
                usernameGroup.id = 'username-group';

                const usernameLabel = document.createElement('label');
                usernameLabel.className = 'form-label';
                usernameLabel.setAttribute('for', 'username');
                usernameLabel.innerHTML = '<strong>Username</strong>';

                const usernameInput = document.createElement('input');
                usernameInput.className = 'form-control';
                usernameInput.type = 'text';
                usernameInput.id = 'username';
                usernameInput.placeholder = 'Enter username';

                usernameGroup.appendChild(usernameLabel);
                usernameGroup.appendChild(usernameInput);

                // Füge das Username-Feld nach dem Address-Feld ein
                const addressGroup = firstCol.querySelector('.form-group');
                if (addressGroup) {
                    addressGroup.parentNode.insertBefore(usernameGroup, addressGroup.nextSibling);
                } else {
                    // Falls das Address-Feld nicht gefunden wird, füge es am Ende ein
                    firstCol.appendChild(usernameGroup);
                }
            }

            // Prüfe, ob das Password-Feld bereits existiert
            const existingPasswordGroup = secondCol.querySelector('#password-group');
            if (!existingPasswordGroup) {
                // Erstelle die Password-Feldgruppe
                const passwordGroup = document.createElement('div');
                passwordGroup.className = 'form-group mb-3';
                passwordGroup.id = 'password-group';

                const passwordLabel = document.createElement('label');
                passwordLabel.className = 'form-label';
                passwordLabel.setAttribute('for', 'password');
                passwordLabel.innerHTML = '<strong>Password</strong>';

                const passwordInput = document.createElement('input');
                passwordInput.className = 'form-control';
                passwordInput.type = 'password';
                passwordInput.id = 'password';
                passwordInput.placeholder = 'Enter password';

                passwordGroup.appendChild(passwordLabel);
                passwordGroup.appendChild(passwordInput);

                // Füge das Password-Feld nach dem Authentication Settings-Feld ein
                const authSettingsGroup = secondCol.querySelector('#select-authentication-settings-1').parentElement;
                if (authSettingsGroup) {
                    authSettingsGroup.parentNode.insertBefore(passwordGroup, authSettingsGroup.nextSibling);
                } else {
                    // Falls das Authentication Settings-Feld nicht gefunden wird, füge es am Ende ein
                    secondCol.appendChild(passwordGroup);
                }
            }
        }

        // Funktion zum Entfernen der Username- und Password-Felder
        function hideCredentials() {
            // Entferne das Username-Feld
            const usernameGroup = opcUaConfig.querySelector('#username-group');
            if (usernameGroup) {
                usernameGroup.remove();
            }

            // Entferne das Password-Feld
            const passwordGroup = opcUaConfig.querySelector('#password-group');
            if (passwordGroup) {
                passwordGroup.remove();
            }
        }

        // Funktion zur Behandlung der Änderung der Authentifizierungseinstellungen
        function handleAuthenticationSettingsChange() {
            const selectedAuth = selectAuthenticationSettings.value;
            // console.log(`Authentication Settings changed to: ${selectedAuth}`);
            if (selectedAuth === 'user-pw') {
                showCredentials();
            } else {
                hideCredentials();
            }
        }

        // Initiale Behandlung basierend auf der aktuellen Auswahl
        handleAuthenticationSettingsChange();

        // Entferne vorherige Event Listener, um doppelte Listener zu vermeiden
        selectAuthenticationSettings.removeEventListener('change', handleAuthenticationSettingsChange);

        // Füge Event Listener hinzu
        selectAuthenticationSettings.addEventListener('change', handleAuthenticationSettingsChange);
    }

    // Save device_id in web storage
    localStorage.setItem('device_id', device_id);
}

function createEmptyRow() {
    const emptyRow = document.createElement('tr');

    // Leere Felder für Datapoint ID
    const idCell = document.createElement('td');
    const idInput = document.createElement('input');
    idInput.type = 'text';
    idInput.className = 'form-control';
    idInput.placeholder = 'Enter ID';
    idCell.appendChild(idInput);
    emptyRow.appendChild(idCell);

    // Leere Felder für Name
    const nameCell = document.createElement('td');
    const nameInput = document.createElement('input');
    nameInput.type = 'text';
    nameInput.className = 'form-control';
    nameInput.placeholder = 'Enter Name';
    nameCell.appendChild(nameInput);
    emptyRow.appendChild(nameCell);

    // Leere Felder für Address / Node ID
    const addressCell = document.createElement('td');
    const addressInput = document.createElement('input');
    addressInput.type = 'text';
    addressInput.className = 'form-control';
    addressInput.placeholder = 'Enter Address / Node ID';
    addressCell.appendChild(addressInput);
    emptyRow.appendChild(addressCell);

    // Action Cell mit Speicher-Button
    const actionCell = document.createElement('td');
    const saveButton = document.createElement('button');
    saveButton.className = 'btn btn-success';
    saveButton.textContent = 'Save';
    saveButton.addEventListener('click', () => {
        saveDatapoint(idInput.value, nameInput.value, addressInput.value);
    });
    actionCell.appendChild(saveButton);
    emptyRow.appendChild(actionCell);

    return emptyRow;
}

function saveDatapoint(id, name, address) {
    if (!name || !address) {
        alert('Please fill all fields before saving!');
        return;
    }

    const datapointId = id ? id : Math.floor(Math.random() * 1000);

    const newRow = document.createElement('tr');

    const idCell = document.createElement('td');
    idCell.textContent = id;
    idCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(idCell);

    // Name
    const nameCell = document.createElement('td');
    nameCell.textContent = name;
    nameCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(nameCell);

    // Address
    const addressCell = document.createElement('td');
    addressCell.textContent = address;
    addressCell.style.color = 'rgb(121, 121, 121)';
    newRow.appendChild(addressCell);

    // Action mit Löschen
    const actionCell = document.createElement('td');
    actionCell.innerHTML = `
                        <a class="btn btnMaterial btn-flat accent btnNoBorders checkboxHover" style="margin-left: 5px;" data-bs-toggle="modal" data-bs-target="#delete-modal">
                            <i class="fas fa-trash btnNoBorders" style="color: #DC3545;"></i>
                        </a>
                    `;
    newRow.appendChild(actionCell);

    // Neue Zeile vor der leeren Zeile hinzufügen
    const tableBody = document.querySelector('#ipi-table tbody');
    const lastRow = tableBody.lastChild; // Leere Zeile
    tableBody.insertBefore(newRow, lastRow);

    // Felder der leeren Zeile zurücksetzen
    const inputs = lastRow.querySelectorAll('input');
    inputs.forEach(input => (input.value = ''));
}
