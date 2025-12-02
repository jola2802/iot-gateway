import onnxruntime as ort
session = ort.InferenceSession("./models/model.onnx")
print("Input shapes:", [input.shape for input in session.get_inputs()])
print("Output shapes:", [output.shape for output in session.get_outputs()])