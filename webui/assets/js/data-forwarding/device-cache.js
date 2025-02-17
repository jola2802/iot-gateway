// Singleton für Device-Cache
const DeviceCache = {
    devices: null,
    
    async getDevices() {
        if (this.devices) {
            return this.devices;
        }

        try {
            const response = await fetch('/api/get-devices-for-routes');
            if (!response.ok) {
                throw new Error('Fehler beim Laden der Geräte');
            }
            const data = await response.json();
            
            if (data && Array.isArray(data.devices)) {
                this.devices = data.devices;
                return this.devices;
            }
            
            throw new Error('Unerwartetes Datenformat');
        } catch (error) {
            console.error('Fehler beim Laden der Geräte:', error);
            return [];
        }
    },

    clearCache() {
        this.devices = null;
    }
}; 