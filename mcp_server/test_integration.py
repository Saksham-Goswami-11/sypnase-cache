import os
import shutil
import time
from ingest import IngestionDaemon
from retrieve import HybridRetriever

def main():
    test_dir = "./knowledge_base_test"
    if os.path.exists(test_dir):
        shutil.rmtree(test_dir)
    os.makedirs(test_dir)

    test_file = os.path.join(test_dir, "onboarding.md")
    
    print("Writing test document...")
    onboarding_content = """
# Onboarding Guide

Welcome to the team! We are building a Local-First Agentic RAG Server.
Your secret Synapse Cache database key is 99-99-99.
Make sure to keep this safe.
Office lunch is at 1 PM.
    """
    with open(test_file, 'w', encoding='utf-8') as f:
        f.write(onboarding_content)

    print("Running IngestionDaemon scan...")
    daemon = IngestionDaemon(directory=test_dir, port=6380)
    daemon.scan_directory()

    print("Running HybridRetriever query...")
    retriever = HybridRetriever(directory=test_dir, port=6380)
    
    # Test semantic query
    results = retriever.retrieve("What is my secret Synapse Cache database key?")
    print(f"Query Results: {results}")

    assert len(results) > 0, "No results returned!"
    top_result = results[0]
    print(f"Top Result source: {top_result['source']}")
    print(f"Top Result RRF Score: {top_result['rrf_score']}")
    assert top_result["source"] == "onboarding.md", "Wrong source file!"
    assert "99-99-99" in top_result["text"], "Expected text content not found!"

    # Test keyword query
    results_kw = retriever.retrieve("lunch")
    assert len(results_kw) > 0, "No results for keyword query!"
    assert "lunch" in results_kw[0]["text"].lower(), "Expected lunch text in response!"

    print("Cleaning up test directory...")
    shutil.rmtree(test_dir)

    print("All integration tests passed successfully!")

if __name__ == "__main__":
    main()
