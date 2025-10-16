#!/usr/bin/env python3
"""
Bildverarbeitungs-API mit ONNX-Modell
Empfängt Bilder über HTTP und gibt verarbeitete Bilder zurück
"""

import cv2
import numpy as np
from PIL import Image
import os
from rembg import remove
import onnxruntime as ort
from flask import Flask, request, jsonify
import base64
import io
from datetime import datetime

# Pfad zum ONNX-Modell
MODEL_PATH = "./models/model.onnx"

# Flask-App initialisieren
app = Flask(__name__)

def process_image_from_bytes(image_bytes: bytes) -> dict:
    try:        
        # Konvertiere Bytes zu OpenCV-Image
        nparr = np.frombuffer(image_bytes, np.uint8)
        
        # Dekodiere zuerst das Bild
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        img_original = img

        if img is None:
            return {"error": "Fehler beim Dekodieren des Bildes"}
        
        img = cv2.cvtColor(
            remove(img, bgcolor=(255, 255, 255, 255)), cv2.COLOR_BGR2GRAY
        )

        # Schneide das Bild entsprechend zu (512x512)
        img = crop_image(img)

        # Speichere das Bild auf der Festplatte
        # cv2.imwrite("0_resized.png", img)

        # Führe ONNX-Modell-Inferenz durch
        onnx_result = run_onnx_model_in_memory(img)

        # Speichere das Bild auf der Festplatte
        # cv2.imwrite("0_onnx_result.png", onnx_result)

        if onnx_result is None:
            return {"error": "ONNX-Modell-Inferenz fehlgeschlagen"}
        
        # Konvertiere Bilder zu Base64
        processed_base64 = image_to_base64(img)
        onnx_result_base64 = image_to_base64(onnx_result)

        # Speichere die Bilder auf der Festplatte (verwende die numpy arrays, nicht die Base64-Strings)
        # cv2.imwrite("0_processed_image.png", img)

        # Rotiere das onnx_result numpy array um 90 grad nach rechts und speichere es
        # onnx_result_rotated = np.rot90(onnx_result, 1)
        
        return {
            "processed_image": processed_base64,
            "image": onnx_result_base64,
            "original_shape": tuple(img_original.shape),
            "processed_shape": tuple(onnx_result.shape) if isinstance(onnx_result, np.ndarray) else "N/A",
        }
        
    except Exception as e:
        print(f"❌ Fehler bei der Bildverarbeitung: {e}")
        return {"error": f"Fehler bei der Bildverarbeitung: {str(e)}"}

def image_to_base64(image_array: np.ndarray) -> str:
    # Konvertiere zu PNG-Bytes
    _, buffer = cv2.imencode('.png', image_array)
    # Konvertiere zu Base64
    img_base64 = base64.b64encode(buffer).decode('utf-8')
    return img_base64

def crop_image(image_array: np.ndarray) -> np.ndarray:
    # Berechne die Mitte der x-Achse und nimm 256 Pixel auf beiden Seiten
    x_center = image_array.shape[1] // 2
    x_start = x_center - 256
    x_end = x_center + 256
        
    # Nimm die unteren 512 Pixel von der y-Achse
    y_start = image_array.shape[0] - 512
    y_end = image_array.shape[0]
        
    # Schneide das Bild entsprechend zu
    resized = image_array[y_start:y_end, x_start:x_end]
    return resized

def run_onnx_model_in_memory(image_array: np.ndarray) -> np.ndarray:
    try:            
        session = ort.InferenceSession(MODEL_PATH)
        
        # Normalisiere das Bild (0-1) und konvertiere zu float32
        img_normalized = image_array.astype(np.float32) / 255.0

        # Transformiere das Bild (x und y um -90 Grad) VOR dem Reshape
        # img_normalized = np.rot90(img_normalized, -1)
        
        # Reshape für ONNX-Input: [1, 1, 512, 512]
        input_tensor = img_normalized.reshape(1, 1, 512, 512)

        # Führe Inferenz durch
        input_name = session.get_inputs()[0].name
        label_name = session.get_outputs()[0].name
        
        # ONNX-Inferenz
        outputs = session.run(
            # [label_name], 
            # {input_name: input_tensor}
            None,
            {input_name: input_tensor}
        )
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

        # Normalisiere für Bildspeicherung (0-255)
        output_img = (output_img * 255).astype(np.uint8)

        # Rotiere das Ergebnis um 90 grad nach rechts
        # output_img = np.rot90(output_img, 1)
        
        return output_img
        
    except Exception as e:
        print(f"❌ Fehler bei ONNX-Inferenz: {e}")
        return None

@app.route('/process-image', methods=['POST'])
def process_image_endpoint():
    try:
        # print("Request: ", request.data)
        image_bytes = base64.b64decode(request.data)
        file = io.BytesIO(image_bytes)
        
        # Verarbeite das Bild
        result = process_image_from_bytes(image_bytes)
        
        if "error" in result:
            return result
        
        return jsonify(result)
        
    except Exception as e:
        return jsonify({"error": f"Server-Fehler: {str(e)}"}), 500

if __name__ == "__main__":
    print("🚀 Starte Bildverarbeitungs-API...")
    print("=" * 50)
    
    app.run(host='0.0.0.0', port=8087, debug=True)