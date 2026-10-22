// This code runs in the browser. nginx proxies /api/ to the API listener.
document.querySelector("button").addEventListener("click", async () => {
  const result = document.querySelector("output");
  try {
    const response = await fetch("/api/jobs", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ task: "example" }),
    });
    if (!response.ok) throw new Error(`Request failed: ${response.status}`);
    result.textContent = (await response.json()).id;
  } catch (error) {
    result.textContent = error.message;
  }
});
