import json
import random
import math

chunks = [
    "HR Policy 1: Employees are entitled to 20 days of paid time off per year.",
    "HR Policy 2: Sick leave is accrued at 1 day per month.",
    "HR Policy 3: The standard work hours are 9 AM to 5 PM, Monday through Friday.",
    "IT Policy 1: Passwords must be changed every 90 days and require special characters.",
    "IT Policy 2: Personal devices are not allowed on the internal secure network.",
    "IT Policy 3: All company laptops must have full-disk encryption enabled.",
    "Benefits 1: The company matches 401(k) contributions up to 5% of your salary.",
    "Benefits 2: Health insurance covers dental and vision starting from day one.",
    "Travel 1: Flight bookings must be made at least 14 days in advance.",
    "Travel 2: The daily meal allowance for business trips is $75.",
]

# Generate more to get to 50
for i in range(11, 51):
    chunks.append(f"General Policy {i}: This is a placeholder policy document covering standard operational procedures.")

def generate_random_vector(dim=1536):
    vec = [random.uniform(-1, 1) for _ in range(dim)]
    norm = math.sqrt(sum(v*v for v in vec))
    return [v / norm for v in vec]

data = []
for i, text in enumerate(chunks):
    # To make queries somewhat work deterministically for the first 10, 
    # we just use random but they won't match semantically. 
    # This is just a structural demo.
    data.append({
        "id": f"chunk:{i+1}",
        "text": text,
        "embedding": generate_random_vector(1536),
        "metadata": {"source": "employee_handbook.pdf"}
    })

with open("sample_embeddings.json", "w") as f:
    json.dump(data, f, indent=2)

print("Generated sample_embeddings.json")
