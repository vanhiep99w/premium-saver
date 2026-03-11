function getCsrfToken() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.content : '';
}

function showAddUser() {
    document.getElementById('add-user-modal').style.display = 'flex';
    document.getElementById('new-user-name').focus();
}

function hideAddUser() {
    document.getElementById('add-user-modal').style.display = 'none';
    document.getElementById('new-user-name').value = '';
}

function hideAPIKey() {
    document.getElementById('api-key-modal').style.display = 'none';
    location.reload();
}

function copyAPIKey() {
    const key = document.getElementById('api-key-value').textContent;
    navigator.clipboard.writeText(key);
    document.querySelector('.btn-copy').textContent = 'Copied!';
}

async function createUser() {
    const name = document.getElementById('new-user-name').value.trim();
    if (!name) return;

    const formData = new URLSearchParams();
    formData.append('name', name);
    formData.append('csrf_token', getCsrfToken());

    const resp = await fetch('/admin/users', {
        method: 'POST',
        headers: { 'X-CSRF-Token': getCsrfToken() },
        body: formData,
    });

    if (!resp.ok) {
        alert('Failed to create user');
        return;
    }

    const data = await resp.json();
    hideAddUser();

    document.getElementById('api-key-value').textContent = data.api_key;
    document.getElementById('api-key-modal').style.display = 'flex';
}

async function deleteUser(id, name) {
    if (!confirm('Delete user "' + name + '"? This will also delete all their usage data.')) return;

    const resp = await fetch('/admin/users/' + id, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': getCsrfToken() },
    });

    if (resp.ok) {
        document.getElementById('user-row-' + id).remove();
    } else {
        alert('Failed to delete user');
    }
}

async function toggleActive(id, currentlyActive) {
    const resp = await fetch('/admin/users/' + id, {
        method: 'PATCH',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCsrfToken(),
        },
        body: JSON.stringify({ active: !currentlyActive }),
    });

    if (resp.ok) {
        location.reload();
    } else {
        alert('Failed to update user');
    }
}
