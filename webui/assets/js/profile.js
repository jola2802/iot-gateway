// Funktion, um Profildaten von der REST-API zu laden und in die Felder einzufügen
async function loadProfileData() {
    try {
        const response = await fetch('/api/profile', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        });

        if (!response.ok) {
            throw new Error(`Fehler beim Abrufen der Profildaten: ${response.status}`);
        }

        const profileData = await response.json();

        // Alle Felder ausfüllen
        document.getElementById('company').value = profileData.company || '';
        document.getElementById('email').value = profileData.email || '';
        document.getElementById('name').value = profileData.name || '';
        document.getElementById('username').value = profileData.username || '';
        document.getElementById('address').value = profileData.address || '';
    } catch (error) {
        console.error('Fehler beim Laden der Profildaten:', error);
        alert('Es gab ein Problem beim Laden Ihrer Profildaten. Bitte versuchen Sie es später erneut.');
    }
}

// Funktion für den Save-Button bei User Settings
async function saveUserSettings(event) {
    event.preventDefault();

    const userData = {
        company: document.getElementById('company').value.trim(),
        email: document.getElementById('email').value.trim(),
        username: document.getElementById('username').value.trim(),
        name: document.getElementById('name').value.trim(),
        address: document.getElementById('address').value.trim()
    };

    try {
        const response = await fetch('/api/profile', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(userData)
        });

        if (!response.ok) {
            throw new Error(`Fehler beim Speichern der Einstellungen: ${response.status}`);
        }

        alert('Einstellungen erfolgreich gespeichert.');
    } catch (error) {
        console.error('Fehler beim Speichern der Einstellungen:', error);
        alert('Fehler beim Speichern der Einstellungen. Bitte versuchen Sie es später erneut.');
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
    document.getElementById('save-user-settings').addEventListener('click', saveUserSettings);
});

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
        method: 'PUT',
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
            window.location.href = '/logout';
            window.location.reload();
            window.location.href = '/login';
        } else {
            throw new Error('Failed to change the password. Please try again later.');
        }
    }).catch(function(error) {
        console.error(error);
        alert(error.message);
    });
});