// data-forwarding.js

// Lade Node-RED URL
async function loadNodeRedURLFlow() {
    try {
        const response = await fetch('/api/get-node-red-url');
        const data = await response.json();
        const nodeRedURLFlow = document.getElementById('node-red-data-forwarding-flow');
        if (nodeRedURLFlow) {
            nodeRedURLFlow.src = data.nodeRedURL;
        }
    } catch (error) {
        console.error('Fehler beim Laden der Node-RED URL:', error);
    }
}

// Initialisiere Bilder-Anzeige
function initializeImagesFiles() {
    const imagesFilesContainer = document.getElementById('images-files-container');
    const downloadAllImagesBtn = document.getElementById('download-all-images');
    if (!imagesFilesContainer) return;

    // Bilder vom Server laden
    fetch('/api/images')
        .then(response => response.json())
        .then(data => {
            imagesFilesContainer.innerHTML = '';

            // Sortiere Bilder nach Timestamp (neueste zuerst)
            data.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));

            // Erstelle ein responsives Grid für die Bilder
            const rowDiv = document.createElement('div');
            rowDiv.classList.add('row', 'row-cols-1', 'row-cols-md-3', 'row-cols-lg-5', 'g-3');

            // Zeige maximal 25 Bilder an
            const imagesToShow = data.slice(0, 25);
            
            imagesToShow.forEach((image) => {
                const colDiv = document.createElement('div');
                colDiv.classList.add('col');
                
                const cardDiv = document.createElement('div');
                cardDiv.classList.add('card', 'h-100');
                
                // Bild aus dem Base64-String anzeigen
                const imgElement = document.createElement('img');
                imgElement.src = image.image.startsWith('data:image') ? image.image : 'data:image/png;base64,' + image.image;
                imgElement.alt = 'Bild von ' + image.device;
                imgElement.classList.add('card-img-top', 'img-thumbnail');
                imgElement.style.height = '150px';
                imgElement.style.objectFit = 'cover';
                
                // Klickbar machen für Vollbildansicht
                imgElement.style.cursor = 'pointer';
                imgElement.addEventListener('click', () => {
                    showImageModal(image);
                });
                
                // Karteninhalt
                const cardBody = document.createElement('div');
                cardBody.classList.add('card-body', 'p-2');
                
                // Gerätenamen anzeigen
                const deviceName = document.createElement('h6');
                deviceName.classList.add('card-title');
                deviceName.textContent = image.device;
                
                // Zeitstempel formatieren und anzeigen
                const timestamp = document.createElement('p');
                timestamp.classList.add('card-text', 'small', 'text-muted');
                const date = new Date(image.timestamp);
                timestamp.textContent = date.toLocaleString('de-DE');
                
                // Alles zusammenfügen
                cardBody.appendChild(deviceName);
                cardBody.appendChild(timestamp);
                cardDiv.appendChild(imgElement);
                cardDiv.appendChild(cardBody);
                colDiv.appendChild(cardDiv);
                rowDiv.appendChild(colDiv);
            });
            
            imagesFilesContainer.appendChild(rowDiv);
        })
        .catch(error => {
            console.error('Fehler beim Laden der Bilder:', error);
            imagesFilesContainer.innerHTML = '<div class="alert alert-danger">Fehler beim Laden der Bilder</div>';
        });
    
    // Download-Button-Funktionalität
    if (downloadAllImagesBtn) {
        downloadAllImagesBtn.addEventListener('click', () => {
            window.location.href = '/api/images/download';
        });
    }
}

// Funktion zum Anzeigen eines Bildes im Modal
function showImageModal(image) {
    let modal = document.getElementById('image-preview-modal');
    
    if (!modal) {
        modal = document.createElement('div');
        modal.id = 'image-preview-modal';
        modal.classList.add('modal', 'fade');
        modal.setAttribute('tabindex', '-1');
        modal.setAttribute('role', 'dialog');
        modal.setAttribute('aria-hidden', 'true');
        
        modal.innerHTML = `
            <div class="modal-dialog modal-lg">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">Bildvorschau</h5>
                        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
                    </div>
                    <div class="modal-body text-center">
                        <img id="modal-image" class="img-fluid" alt="Bildvorschau">
                        <div class="mt-2">
                            <p id="modal-device" class="mb-1"></p>
                            <p id="modal-timestamp" class="text-muted small"></p>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Schließen</button>
                        <a id="modal-download" class="btn btn-primary" download>
                            <i class="fas fa-download"></i> Herunterladen
                        </a>
                    </div>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
    }
    
    // Setze die Bildinformationen
    const modalImage = document.getElementById('modal-image');
    const modalDevice = document.getElementById('modal-device');
    const modalTimestamp = document.getElementById('modal-timestamp');
    const modalDownload = document.getElementById('modal-download');
    
    const imageSource = image.image.startsWith('data:image') ? image.image : 'data:image/png;base64,' + image.image;
    
    modalImage.src = imageSource;
    modalDevice.textContent = 'Gerät: ' + image.device;
    
    const date = new Date(image.timestamp);
    modalTimestamp.textContent = 'Aufgenommen am: ' + date.toLocaleString('de-DE');
    
    modalDownload.href = imageSource;
    modalDownload.download = `image_${image.device}_${date.toISOString().split('T')[0]}.png`;
    
    const bsModal = new bootstrap.Modal(modal);
    bsModal.show();
}

// Initialisierung beim Laden der Seite
document.addEventListener('DOMContentLoaded', () => {
    loadNodeRedURLFlow();
    initializeImagesFiles();
});