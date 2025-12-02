import onnx
from onnx import numpy_helper

model = onnx.load("./models/model.onnx")
graph = model.graph

print("=== Inputs ===")
for inp in graph.input:
    print(inp.name, inp.type.tensor_type.shape.dim)

print("\n=== Outputs ===")
for out in graph.output:
    print(out.name, out.type.tensor_type.shape.dim)

print("\n=== Nodes ===")
for node in graph.node:
    print(node.op_type)

print("\n=== Last 10 nodes ===")
for node in graph.node[-10:]:
    print(node.op_type, node.name)

sizes = []
for init in graph.initializer:
    arr = numpy_helper.to_array(init)
    if arr.ndim == 4:  # conv weights
        sizes.append(arr.shape)
        
print("\n=== Conv weight shapes ===")
for s in sizes:
    print(s)
