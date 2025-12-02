import requests
import os
import json
import base64
import numpy as np
import cv2

# Die URL deines laufenden Servers
API_URL = "http://192.168.0.206:8087/process-image"

def test(image_path):    
    # Öffne das Bild und konvertiere es zu Base64
    with open(image_path, "rb") as image_file:
        image_bytes = image_file.read()
        image_base64 = base64.b64encode(image_bytes).decode('utf-8')
    
    # Sende das Bild als Base64 im Request-Body
    response = requests.post(API_URL, data=image_base64, headers={'Content-Type': 'text/plain'})
    
    # Prüfe ob Request erfolgreich war
    if response.status_code != 200:
        print(f"❌ Fehler: {response.status_code} - {response.text}")
        return None

    # Parse JSON Response
    response_data = response.json()
    
    # Prüfe auf Fehler
    if "error" in response_data:
        print(f"❌ API-Fehler: {response_data['error']}")
        return response_data
    
    print("Euklidische Distanz: " + str(response_data["euclidean_distance"]))
    
    # Speichere das verarbeitete Bild (Standard)
    if "processed_image" in response_data:
        with open(image_path.replace(".png", "_result.png"), "wb") as f:
            f.write(base64.b64decode(response_data["processed_image"]))
    
    # Speichere das Feature-Bild (Standard)
    if "feature_image" in response_data:
        with open(image_path.replace(".png", "_feature.png"), "wb") as f:
            f.write(base64.b64decode(response_data["feature_image"]))
    
    # Kombiniere beide Methoden in einem Bild, falls Vergleichsdaten vorhanden
    if "comparison" in response_data:
        comparison = response_data["comparison"]
        
        # Kombiniere Feature-Bilder nebeneinander
        if "feature_image" in comparison["withoutbg"] and "feature_image" in comparison["rembg"]:
            withoutbg_feature = base64.b64decode(comparison["withoutbg"]["feature_image"])
            rembg_feature = base64.b64decode(comparison["rembg"]["feature_image"])
            
            # Dekodiere beide Bilder
            img1 = cv2.imdecode(np.frombuffer(withoutbg_feature, np.uint8), cv2.IMREAD_GRAYSCALE)
            img2 = cv2.imdecode(np.frombuffer(rembg_feature, np.uint8), cv2.IMREAD_GRAYSCALE)
            
            if img1 is not None and img2 is not None:
                # Stelle sicher, dass beide Bilder die gleiche Höhe haben
                if img1.shape[0] != img2.shape[0]:
                    h = min(img1.shape[0], img2.shape[0])
                    img1 = img1[:h, :]
                    img2 = img2[:h, :]
                
                # Kombiniere nebeneinander
                combined_feature = np.hstack([img1, img2])
                
                # Füge Text-Labels hinzu
                cv2.putText(combined_feature, f"WithoutBG ({comparison['withoutbg']['euclidean_distance']:.1f})", 
                           (10, 30), cv2.FONT_HERSHEY_SIMPLEX, 0.7, 255, 2)
                cv2.putText(combined_feature, f"rembg ({comparison['rembg']['euclidean_distance']:.1f})", 
                           (img1.shape[1] + 10, 30), cv2.FONT_HERSHEY_SIMPLEX, 0.7, 255, 2)
                
                # Speichere kombiniertes Feature-Bild
                cv2.imwrite(image_path.replace(".png", "_feature_comparison.png"), combined_feature)
                print(f"  ✅ Vergleich Feature-Bild gespeichert")
        
        # Kombiniere Result-Bilder nebeneinander
        if "processed_image" in comparison["withoutbg"] and "processed_image" in comparison["rembg"]:
            withoutbg_result = base64.b64decode(comparison["withoutbg"]["processed_image"])
            rembg_result = base64.b64decode(comparison["rembg"]["processed_image"])
            
            # Dekodiere beide Bilder
            img1 = cv2.imdecode(np.frombuffer(withoutbg_result, np.uint8), cv2.IMREAD_GRAYSCALE)
            img2 = cv2.imdecode(np.frombuffer(rembg_result, np.uint8), cv2.IMREAD_GRAYSCALE)
            
            if img1 is not None and img2 is not None:
                # Stelle sicher, dass beide Bilder die gleiche Höhe haben
                if img1.shape[0] != img2.shape[0]:
                    h = min(img1.shape[0], img2.shape[0])
                    img1 = img1[:h, :]
                    img2 = img2[:h, :]
                
                # Kombiniere nebeneinander
                combined_result = np.hstack([img1, img2])
                
                # Füge Text-Labels hinzu
                cv2.putText(combined_result, f"WithoutBG ({comparison['withoutbg']['euclidean_distance']:.1f})", 
                           (10, 30), cv2.FONT_HERSHEY_SIMPLEX, 0.7, 255, 2)
                cv2.putText(combined_result, f"rembg ({comparison['rembg']['euclidean_distance']:.1f})", 
                           (img1.shape[1] + 10, 30), cv2.FONT_HERSHEY_SIMPLEX, 0.7, 255, 2)
                
                # Speichere kombiniertes Result-Bild
                cv2.imwrite(image_path.replace(".png", "_result_comparison.png"), combined_result)
                print(f"  ✅ Vergleich Result-Bild gespeichert")
        
        # Zeige Vergleichsergebnis
        better = comparison.get("better_method", "unknown")
        diff = comparison.get("difference", 0)
        print(f"  📊 Besser: {better.upper()} (Differenz: {diff:.2f})")

    return response_data

def test_all_images():
    for image in os.listdir("martin/frames"):
        print(f"Testing {image}...")
        result = test(f"martin/frames/{image}")

if __name__ == "__main__":
    print("🚀 Starte API Test...")
    print("=" * 50)
    test_all_images()
    print("=" * 50)