// Funktion, um Profildaten von der REST-API zu laden und in die Felder einzufügen
async function loadProfileData() {
    try {
        // API-Aufruf
        const response = await fetch('/api/profile', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        // Überprüfen, ob der API-Aufruf erfolgreich war
        if (!response.ok) {
            throw new Error(`Fehler beim Abrufen der Profildaten: ${response.status}`);
        }

        // Daten in JSON umwandeln
        const profileData = await response.json();

        // Felder für User Settings ausfüllen
        document.getElementById('company').value = profileData.company || '';
        // document.getElementById('email').value = profileData.email || '';
        document.getElementById('name').value = profileData.name || '';
        document.getElementById('username').value = profileData.username || '';

        // Felder für Contact Settings ausfüllen
        document.getElementById('address').value = profileData.address || '';
        // document.getElementById('city').value = profileData.city || '';
        // document.getElementById('country').value = profileData.country || '';
    } catch (error) {
        console.error('Fehler beim Laden der Profildaten:', error);
        alert('Es gab ein Problem beim Laden Ihrer Profildaten. Bitte versuchen Sie es später erneut.');
    }
}

// Funktion für den Save-Button bei User Settings
async function saveUserSettings(event) {
    event.preventDefault(); // Verhindert das Standard-Formularverhalten

    // Daten sammeln
    const userData = {
        company: document.getElementById('company').value.trim(),
        // email: document.getElementById('email').value.trim(),
        username: document.getElementById('username').value.trim(),
        name: document.getElementById('name').value.trim()
    };

    try {
        // API-Aufruf
        const response = await fetch('/api/profile', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(userData)
        });

        if (!response.ok) {
            throw new Error(`Fehler beim Speichern der User Settings: ${userData} mit Response ${response.status}`);
        }

        alert('User Settings erfolgreich gespeichert.');
    } catch (error) {
        console.error(`Fehler beim Speichern der User Settings:' ${userData} mit Response`, error);
        alert('Fehler beim Speichern der User Settings. Bitte versuchen Sie es später erneut.');
    }
}

// Funktion für den Save-Button bei Contact Settings
async function saveContactSettings(event) {
    event.preventDefault(); // Verhindert das Standard-Formularverhalten

    // Daten sammeln
    const contactData = {
        address: document.getElementById('address').value.trim(),
        city: document.getElementById('city').value.trim(),
        country: document.getElementById('country').value.trim()
    };

    try {
        // API-Aufruf
        const response = await fetch('/api/profile/contact', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(contactData)
        });

        if (!response.ok) {
            throw new Error(`Fehler beim Speichern der Contact Settings: ${contactData} mit Response ${response.status}`);
        }

        alert('Contact Settings erfolgreich gespeichert.');
    } catch (error) {
        console.error(`Fehler beim Speichern der Contact Settings: ${contactData} mit Response`, error);
        alert(`Fehler beim Speichern der Contact Settings ${contactData}. Bitte versuchen Sie es später erneut.`);
    }
}

// Beim Laden der Seite die Funktion aufrufen
document.addEventListener('DOMContentLoaded', () => {
    loadProfileData();
});

document.getElementById('save-user-settings').addEventListener('click', saveUserSettings);
document.getElementById('save-contact-settings').addEventListener('click', saveContactSettings);

document.getElementById('changePasswordButton').addEventListener('click', function() {
    var currentPassword = document.getElementById('old-password').value;
    var newPassword = document.getElementById('new-password-2').value;

    if (!currentPassword || !newPassword) {
        alert("Both current and new passwords must be provided.");
        return;
    }

    if(newPassword !== document.getElementById('new-password-1').value) {
        alert("The two passwords don't match.");
        return;
    }

    fetch('/api/changePassword', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            currentPassword: currentPassword,
            newPassword: newPassword
        })
    }).then(function(response) {
        if (response.ok) {
            alert('Your password has been successfully changed.');
        } else {
            throw new Error('Failed to change the password. Please try again later.');
        }
    }).catch(function(error) {
        console.error(error);
        alert(error.message);
    });
});