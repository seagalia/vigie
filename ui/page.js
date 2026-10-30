// Fills the endpoints column with the answer of /services for each service, the same answer the clients read.
for (const cell of document.querySelectorAll('td.endpoints')) {
  resolve(cell);
}

async function resolve(cell) {
  const service = cell.dataset.service;
  try {
    const resp = await fetch('services?service_name=' + encodeURIComponent(service));
    if (!resp.ok) {
      show(cell, 'error', (await resp.text()).trim());
      return;
    }
    const endpoints = await resp.json();
    if (endpoints.length === 0) {
      show(cell, 'pending', 'no endpoint');
      return;
    }
    const list = document.createElement('ul');
    for (const endpoint of endpoints) {
      const item = document.createElement('li');
      const address = document.createElement('code');
      address.textContent = endpoint.ip.includes(':') ? `[${endpoint.ip}]:${endpoint.port}` : `${endpoint.ip}:${endpoint.port}`;
      item.append(address);
      if (endpoint.load_balancing_weight > 0) {
        const weight = document.createElement('span');
        weight.className = 'weight';
        weight.textContent = ` weight ${endpoint.load_balancing_weight}`;
        item.append(weight);
      }
      list.append(item);
    }
    cell.replaceChildren(list);
  } catch (err) {
    show(cell, 'error', String(err));
  }
}

function show(cell, className, text) {
  const span = document.createElement('span');
  span.className = className;
  span.textContent = text;
  cell.replaceChildren(span);
}
