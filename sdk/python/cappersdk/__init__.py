"""Capper Control Plane Python SDK.

Usage::

    from cappersdk import CapperClient

    c = CapperClient("http://localhost:8080", token="my-api-token")
    instances = c.instances.list(project="default")
    for inst in instances:
        print(inst["id"], inst["name"], inst["status"])
"""

from __future__ import annotations

from typing import Any, Dict, List, Optional
import urllib.request
import urllib.parse
import json as _json


class APIError(Exception):
    def __init__(self, status: int, body: str) -> None:
        super().__init__(f"Capper API {status}: {body}")
        self.status = status
        self.body = body


class _Resource:
    def __init__(self, client: "CapperClient") -> None:
        self._c = client


class InstancesResource(_Resource):
    def list(self, project: str = "") -> List[Dict[str, Any]]:
        path = "instances"
        if project:
            path += f"?project={urllib.parse.quote(project)}"
        return self._c._get(path).get("instances", [])

    def get(self, id: str) -> Dict[str, Any]:
        return self._c._get(f"instances/{id}")

    def start(self, id: str) -> None:
        self._c._post(f"instances/{id}/start")

    def stop(self, id: str) -> None:
        self._c._post(f"instances/{id}/stop")

    def delete(self, id: str) -> None:
        self._c._delete(f"instances/{id}")


class NetworksResource(_Resource):
    def list(self, project: str = "") -> List[Dict[str, Any]]:
        path = "networks"
        if project:
            path += f"?project={urllib.parse.quote(project)}"
        return self._c._get(path).get("networks", [])


class ImagesResource(_Resource):
    def list(self) -> List[Dict[str, Any]]:
        return self._c._get("images").get("images", [])

    def delete(self, name: str) -> None:
        self._c._delete(f"images/{name}")


class DNSResource(_Resource):
    def list_zones(self) -> List[Dict[str, Any]]:
        return self._c._get("dns/zones").get("zones", [])


class LBResource(_Resource):
    def list(self, project: str = "") -> List[Dict[str, Any]]:
        path = "lb"
        if project:
            path += f"?project={urllib.parse.quote(project)}"
        return self._c._get(path).get("loadBalancers", [])


class SearchResource(_Resource):
    def search(
        self,
        q: str = "",
        project: str = "",
        label: str = "",
        type_filter: str = "",
    ) -> List[Dict[str, Any]]:
        params: Dict[str, str] = {}
        if q:
            params["q"] = q
        if project:
            params["project"] = project
        if label:
            params["label"] = label
        if type_filter:
            params["type"] = type_filter
        path = "search"
        if params:
            path += "?" + urllib.parse.urlencode(params)
        return self._c._get(path).get("results", [])


class CapperClient:
    """Client for the Capper Control Plane REST API."""

    def __init__(self, base_url: str, token: str = "") -> None:
        self._base = base_url.rstrip("/") + "/api/v1"
        self._token = token
        self.instances = InstancesResource(self)
        self.networks = NetworksResource(self)
        self.images = ImagesResource(self)
        self.dns = DNSResource(self)
        self.lb = LBResource(self)
        self.search = SearchResource(self)

    # ---- low-level HTTP helpers --------------------------------------------

    def _request(self, method: str, path: str, body: Optional[Any] = None) -> Any:
        url = f"{self._base}/{path}"
        data = _json.dumps(body).encode() if body is not None else None
        headers = {"Accept": "application/json"}
        if self._token:
            headers["Authorization"] = f"Bearer {self._token}"
        if data is not None:
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req) as resp:
                raw = resp.read()
                return _json.loads(raw) if raw else {}
        except urllib.error.HTTPError as e:
            raise APIError(e.code, e.read().decode()) from e

    def _get(self, path: str) -> Any:
        return self._request("GET", path)

    def _post(self, path: str, body: Optional[Any] = None) -> Any:
        return self._request("POST", path, body)

    def _delete(self, path: str) -> Any:
        return self._request("DELETE", path)
