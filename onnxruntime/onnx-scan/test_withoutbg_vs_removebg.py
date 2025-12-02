#!/usr/bin/env python3
"""
Bildverarbeitungs-API mit ONNX-Modell
Empfängt Bilder über HTTP und gibt verarbeitete Bilder zurück
"""

import cv2
import numpy as np
from rembg import remove, new_session
from PIL import Image
import onnxruntime as ort
from flask import Flask, request, jsonify
import base64
import time
from withoutbg import WithoutBG

# Pfad zum ONNX-Modell
MODEL_PATH = "./models/model.onnx"

# Flask-App initialisieren
app = Flask(__name__)

# Globale Variablen für wiederverwendbare Sessions (werden beim Start geladen)
onnx_session = None
onnx_input_name = None
rembg_session = None


def process_image_from_bytes(image_bytes: bytes) -> dict:
    try:
        # Decode
        img_cv = cv2.imdecode(np.frombuffer(image_bytes, np.uint8), cv2.IMREAD_COLOR)
        if img_cv is None:
            return {"error": "Fehler beim Dekodieren des Bildes"}

        # Führe beide Methoden durch und vergleiche
        # Methode 1: WithoutBG
        feature_withoutbg = preprocess_image_withoutbg(img_cv)
        onnx_out_withoutbg = run_onnx_model_in_memory(feature_withoutbg)
        
        if onnx_out_withoutbg is None:
            return {"error": "ONNX fehler (WithoutBG)"}
        
        if feature_withoutbg.shape != onnx_out_withoutbg.shape:
            onnx_out_withoutbg = cv2.resize(onnx_out_withoutbg, (feature_withoutbg.shape[1], feature_withoutbg.shape[0]))
        
        diff_withoutbg = feature_withoutbg.astype(np.float32) - onnx_out_withoutbg.astype(np.float32)
        euclidean_distance_withoutbg = float(np.linalg.norm(diff_withoutbg))
        
        # Methode 2: rembg
        feature_rembg = preprocess_image_rembg(img_cv)
        onnx_out_rembg = run_onnx_model_in_memory(feature_rembg)
        
        if onnx_out_rembg is None:
            return {"error": "ONNX fehler (rembg)"}
        
        if feature_rembg.shape != onnx_out_rembg.shape:
            onnx_out_rembg = cv2.resize(onnx_out_rembg, (feature_rembg.shape[1], feature_rembg.shape[0]))
        
        diff_rembg = feature_rembg.astype(np.float32) - onnx_out_rembg.astype(np.float32)
        euclidean_distance_rembg = float(np.linalg.norm(diff_rembg))
        
        # Vergleich ausgeben
        print("=" * 60)
        print("VERGLEICH DER METHODEN:")
        print("=" * 60)
        print(f"WithoutBG - Euklidische Distanz: {euclidean_distance_withoutbg:.2f}")
        print(f"rembg     - Euklidische Distanz: {euclidean_distance_rembg:.2f}")
        print("-" * 60)
        if euclidean_distance_withoutbg < euclidean_distance_rembg:
            print(f"✅ WithoutBG ist BESSER (Differenz: {euclidean_distance_rembg - euclidean_distance_withoutbg:.2f})")
        elif euclidean_distance_rembg < euclidean_distance_withoutbg:
            print(f"✅ rembg ist BESSER (Differenz: {euclidean_distance_withoutbg - euclidean_distance_rembg:.2f})")
        else:
            print("⚖️  Beide Methoden sind gleich gut")
        print("=" * 60)

        # Verwende WithoutBG als Standard (kann später geändert werden)
        return {
            "processed_image": image_to_base64(onnx_out_withoutbg),
            "feature_image": image_to_base64(feature_withoutbg),
            "euclidean_distance": euclidean_distance_withoutbg,
            "comparison": {
                "withoutbg": {
                    "euclidean_distance": euclidean_distance_withoutbg,
                    "processed_image": image_to_base64(onnx_out_withoutbg),
                    "feature_image": image_to_base64(feature_withoutbg)
                },
                "rembg": {
                    "euclidean_distance": euclidean_distance_rembg,
                    "processed_image": image_to_base64(onnx_out_rembg),
                    "feature_image": image_to_base64(feature_rembg)
                },
                "better_method": "withoutbg" if euclidean_distance_withoutbg < euclidean_distance_rembg else "rembg",
                "difference": abs(euclidean_distance_withoutbg - euclidean_distance_rembg)
            }
        }

    except Exception as e:
        return {"error": str(e)}


def preprocess_image_withoutbg(img: np.ndarray) -> np.ndarray:
    """
    Preprocessing mit WithoutBG
    """
    try:
        # WithoutBG erwartet ein PIL.Image, nicht ein NumPy-Array
        # Konvertiere NumPy-Array (BGR) zu PIL.Image (RGB)
        img_rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
        img_pil = Image.fromarray(img_rgb)
        
        # Hintergrund entfernen
        withoutbg = WithoutBG.opensource()
        img_removed = withoutbg.remove_background(img_pil)
        
        # Konvertiere PIL.Image zurück zu NumPy-Array
        if isinstance(img_removed, Image.Image):
            img_array = np.array(img_removed)
            
            # WithoutBG gibt normalerweise RGBA zurück (mit Alpha-Kanal)
            if len(img_array.shape) == 3 and img_array.shape[2] == 4:
                # RGBA-Bild: Konvertiere transparenten Hintergrund zu weiß
                # Erstelle weißes Hintergrundbild
                white_bg = np.ones((img_array.shape[0], img_array.shape[1], 3), dtype=np.uint8) * 255
                
                # Extrahiere RGB und Alpha-Kanal
                rgb = img_array[:, :, :3]
                alpha = img_array[:, :, 3:4] / 255.0
                
                # Blende RGB über weißen Hintergrund
                img = (rgb * alpha + white_bg * (1 - alpha)).astype(np.uint8)
                
                # RGB zu BGR für OpenCV
                img = cv2.cvtColor(img, cv2.COLOR_RGB2BGR)
            elif len(img_array.shape) == 3 and img_array.shape[2] == 3:
                # RGB-Bild: Konvertiere zu BGR
                img = cv2.cvtColor(img_array, cv2.COLOR_RGB2BGR)
            else:
                img = img_array
        else:
            img = img_removed
    except Exception as e:
        # Bei Fehler: Exception werfen, damit process_image_from_bytes es abfangen kann
        raise ValueError(f"Background removal failed (WithoutBG): {str(e)}")

    # Graustufe
    img = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    # Crop auf 512x512
    img = crop_image(img)

    return img


def preprocess_image_rembg(img: np.ndarray) -> np.ndarray:
    """
    Preprocessing mit rembg
    """
    global rembg_session
    
    # Hintergrund entfernen mit rembg
    if rembg_session is None:
        img_removed = remove(img, bgcolor=(255, 255, 255, 255))
    else:
        img_removed = remove(img, session=rembg_session, bgcolor=(255, 255, 255, 255))
    
    # Konvertiere PIL.Image zu NumPy-Array falls nötig
    if isinstance(img_removed, Image.Image):
        img = np.array(img_removed)
        # PIL gibt RGB zurück, OpenCV braucht BGR
        if len(img.shape) == 3 and img.shape[2] == 3:
            img = cv2.cvtColor(img, cv2.COLOR_RGB2BGR)
    else:
        img = img_removed

    # Graustufe
    img = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

    # Crop auf 512x512
    img = crop_image(img)

    return img




def image_to_base64(image_array: np.ndarray) -> str:
    """Konvertiert NumPy-Array zu Base64-String (optimiert)"""
    # PNG mit optimierten Parametern für bessere Performance
    success, buffer = cv2.imencode('.png', image_array, [cv2.IMWRITE_PNG_COMPRESSION, 1])
    if not success:
        raise ValueError("Fehler beim Kodieren des Bildes")
    # Direktes base64-Encoding ohne Zwischenschritt
    return base64.b64encode(buffer.tobytes()).decode('utf-8')


def crop_image(image_array: np.ndarray) -> np.ndarray:
    """
    Cropt das Bild auf 512x512.
    Falls das Bild zu klein ist oder der Crop außerhalb liegt, wird es auf 512x512 resized.
    """
    height, width = image_array.shape[:2]

    # Berechne die Mitte der x-Achse und nimm 256 Pixel auf beiden Seiten
    x_center = width // 2
    x_start = max(0, x_center - 256)
    x_end = min(width, x_center + 256)

    # # Nimm die unteren 512 Pixel von der y-Achse
    y_start = max(0, height - 512)
    y_end = height

    # Nimm die oberen 512 Pixel von der y-Achse
    # y_start = 0
    # y_end = 512

    # Schneide das Bild entsprechend zu
    cropped = image_array[y_start:y_end, x_start:x_end]

    # Stelle sicher, dass das Ergebnis genau 512x512 ist
    # Falls der Crop kleiner ist, resize es auf 512x512
    if cropped.shape[0] != 512 or cropped.shape[1] != 512:
        cropped = cv2.resize(cropped, (512, 512), interpolation=cv2.INTER_LINEAR)

    return cropped


def run_onnx_model_in_memory(image_array: np.ndarray) -> np.ndarray:
    global onnx_session, onnx_input_name

    try:
        # Verwende globale Session (wird nur einmal geladen)
        if onnx_session is None:
            raise RuntimeError("ONNX-Session wurde nicht initialisiert!")

        # Normalisiere das Bild (-1, 1) anstatt (0, 1)
        input_tensor = (image_array.astype(np.float32) / 127.5 - 1.0).reshape(1, 1, 512, 512)
        outputs = onnx_session.run(None, {onnx_input_name: input_tensor})
        result = outputs[0]

        # Verarbeite das ONNX-Ergebnis
        if len(result.shape) == 4 and result.shape[1] == 1:
            # Reshape von [1, 1, 512, 512] zu [512, 512]
            output_img = result[0, 0, :, :]
        elif len(result.shape) == 3 and result.shape[0] == 1:
            # Reshape von [1, 512, 512] zu [512, 512]
            output_img = result[0, :, :]
        else:
            output_img = result

        # Denormalisiere für Bildspeicherung (0-255)
        output_img = (output_img + 1.0) / 2.0
        output_img = (output_img * 255).clip(0, 255).astype(np.uint8)

        return output_img

    except Exception as e:
        print(f"❌ Fehler bei ONNX-Inferenz: {e}")
        return None


@app.route('/process-image', methods=['POST'])
def process_image_endpoint():
    # Zeitmessung starten
    start_time = time.perf_counter()

    try:
        # Base64 dekodieren und direkt verarbeiten
        image_bytes = base64.b64decode(request.data)
        # Verarbeite das Bild
        result = process_image_from_bytes(image_bytes)
        # Zeitmessung beenden
        end_time = time.perf_counter()
        processing_time = end_time - start_time

        if "error" in result:
            # Auch bei Fehlern die Zeit zurückgeben
            result["processing_time_seconds"] = processing_time
            print(f"⏱️ Verarbeitungszeit: {processing_time:.2f} Sekunden (Fehler)")
            return result

        # Verarbeitungszeit zur Antwort hinzufügen
        result["processing_time_seconds"] = processing_time
        print(f"⏱️ Verarbeitungszeit: {processing_time:.2f} Sekunden")
        return jsonify(result)

    except Exception as e:
        # Auch bei Exceptions die Zeit messen
        end_time = time.perf_counter()
        processing_time = end_time - start_time
        print(f"⏱️ Verarbeitungszeit: {processing_time:.4f} Sekunden (Exception)")
        return jsonify({"error": f"Server-Fehler: {str(e)}", "processing_time_seconds": processing_time}), 500


def initialize_models():
    """Lädt alle Modelle beim Start der Anwendung"""
    global onnx_session, onnx_input_name, rembg_session

    print("📦 Lade ONNX-Modell...")
    try:
        # ONNX-Session mit CPU-Provider laden
        onnx_session = ort.InferenceSession(MODEL_PATH, providers=['CPUExecutionProvider'])
        onnx_input_name = onnx_session.get_inputs()[0].name
        print(f"✅ ONNX-Modell geladen (Input: {onnx_input_name})")
    except Exception as e:
        print(f"❌ Fehler beim Laden des ONNX-Modells: {e}")
        raise

    print("📦 Initialisiere rembg Session...")
    try:
        # rembg Session initialisieren (optional, aber beschleunigt die Verarbeitung)
        rembg_session = new_session()
        print("✅ rembg Session initialisiert")
    except Exception as e:
        print(f"⚠️ rembg Session konnte nicht initialisiert werden: {e}")
        print(" (Verwende Standard-Modus, kann etwas langsamer sein)")
        rembg_session = None


if __name__ == "__main__":
    print("🚀 Starte Bildverarbeitungs-API...")
    print("=" * 50)

    # Modelle beim Start laden
    initialize_models()

    print("=" * 50)
    print("🌐 Server startet auf http://0.0.0.0:8087")
    print("=" * 50)

    app.run(host='0.0.0.0', port=8087, debug=True)