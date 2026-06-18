"""Example: list running instances using the Capper Python SDK."""
from cappersdk import CapperClient

c = CapperClient("http://localhost:8080", token="your-token-here")

instances = c.instances.list(project="default")
print(f"Found {len(instances)} instance(s):")
for inst in instances:
    print(f"  {inst['name']} ({inst['id']})  status={inst.get('status', '?')}")

# Search for instances with a specific label
results = c.search.search(label="role=web", type_filter="instances")
print(f"\nWeb instances: {len(results)}")
