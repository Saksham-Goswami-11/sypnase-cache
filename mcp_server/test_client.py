from client import SynapseClient

def main():
    print("Connecting to Synapse Cache on localhost:6380...")
    c = SynapseClient(port=6380)

    print("Testing SET/GET...")
    ok = c.set("test_key", "Hello from Python!")
    print(f"SET status: {ok}")

    val = c.get("test_key")
    print(f"GET value: '{val}'")
    assert val == "Hello from Python!"

    print("Testing VSET...")
    ok = c.vset("docs", "chunk_1", [0.1, 0.2, 0.3], {"source": "test.txt", "author": "Saksham"})
    print(f"VSET status: {ok}")

    print("Testing VSIMILARITY...")
    results = c.vsimilarity("docs", [0.1, 0.2, 0.3], 1)
    print(f"VSIMILARITY results: {results}")
    assert len(results) == 1
    assert results[0]["id"] == "chunk_1"
    assert abs(results[0]["score"] - 1.0) < 1e-4
    assert results[0]["metadata"]["source"] == "test.txt"

    print("All Python client tests passed!")

if __name__ == "__main__":
    main()
