// Lade Node-RED URL
async function loadNodeRedURLFlow() {
    try {
        const currentUrl = window.location.href;
        const baseUrl = currentUrl.split('/').slice(0, -1).join('/');
        const nodeRedURLFlow = document.getElementById('node-red-data-forwarding-flow');
        if (nodeRedURLFlow) {
            nodeRedURLFlow.src = "http://127.0.0.1:1880";
        }
    } catch (error) {
        console.error('Fehler beim Laden der Node-RED URL:', error);
    }
}

// Initialisierung beim Laden der Seite
document.addEventListener('DOMContentLoaded', () => {
    loadNodeRedURLFlow();
});