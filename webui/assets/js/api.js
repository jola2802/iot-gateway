function getHostInfo() {
    const hostname = window.location.hostname;
    const port = window.location.port || (window.location.protocol === 'https:' ? '443' : '80');

    // hostname = "192.168.0.84"
    const BASE_PATH = `https://${hostname}/nodered/`;
    const WS_PATH = `wss://${hostname}/nodered/`;

    console.log('BASE_PATH:', BASE_PATH);
    console.log('WS_PATH:', WS_PATH);

    return { BASE_PATH, WS_PATH };
}

// Beispielaufruf der Funktion
const { BASE_PATH, WS_PATH } = getHostInfo();
// Verwende BASE_PATH und WS_PATH weiter in deinem Skript
