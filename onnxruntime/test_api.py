import requests
import os
import json
import base64
import numpy as np
import cv2

# Die URL deines laufenden Servers
API_URL = "http://localhost:8089/process-image"

def test(image_path):    
    # Öffne das Bild und sende es als Form-Data
    with open(image_path, "rb") as image_file:
        files = {"image": image_file}
        response = requests.post(API_URL, files=files)

        # print("Response: " + response.text)

        response = json.loads(response.text)
        # print("Img Shape: " + str(response["original_shape"]))
        # print("Processed Img Shape: " + str(response["processed_shape"]))
        
        # save base64 image to file
        with open(image_path.replace(".png", "_result.png"), "wb") as f:
            f.write(base64.b64decode(response["image"]))
        
        # with open(image_path.replace(".png", "_processed.png"), "wb") as f:
        #     f.write(base64.b64decode(response["processed_image"]))

        return response

def test_all_images():
    for image in os.listdir("martin/frames"):
        print(f"Testing {image}...")
        result = test(f"martin/frames/{image}")

if __name__ == "__main__":
    print("🚀 Starte API Test...")
    print("=" * 50)
    # Teste das Modell direkt (verwendet model.onnx)
    # test()
    test_all_images()
    print("=" * 50)