function getCsrfToken() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.content : '';
}

function showAddUser() {
    const modal = document.getElementById('add-user-modal');
    modal.style.display = 'flex';
    requestAnimationFrame(() => modal.classList.add('show'));
    document.getElementById('new-user-name').focus();
}

function hideAddUser() {
    const modal = document.getElementById('add-user-modal');
    modal.classList.remove('show');
    setTimeout(() => { modal.style.display = 'none'; }, 200);
    document.getElementById('new-user-name').value = '';
}

function hideAPIKey() {
    const modal = document.getElementById('api-key-modal');
    modal.classList.remove('show');
    setTimeout(() => { modal.style.display = 'none'; }, 200);
    location.reload();
}

async function copyToClipboard(text, btn, label) {
    try {
        if (navigator.clipboard && window.isSecureContext) {
            await navigator.clipboard.writeText(text);
        } else {
            const ta = document.createElement('textarea');
            ta.value = text;
            ta.style.position = 'fixed';
            ta.style.left = '-9999px';
            document.body.appendChild(ta);
            ta.select();
            document.execCommand('copy');
            document.body.removeChild(ta);
        }
        const orig = btn.textContent;
        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {
            btn.textContent = label || orig;
            btn.classList.remove('copied');
        }, 2000);
    } catch (err) {
        btn.textContent = 'Failed';
        setTimeout(() => { btn.textContent = label || 'Copy'; }, 2000);
    }
}

async function copyAPIKey() {
    const key = document.getElementById('api-key-value').textContent;
    const btn = document.querySelector('#api-key-modal .btn-copy');
    await copyToClipboard(key, btn, 'Copy');
}

async function copyText(text, btn) {
    await copyToClipboard(text, btn, btn.textContent);
}

async function createUser() {
    const name = document.getElementById('new-user-name').value.trim();
    if (!name) return;

    const btn = document.querySelector('#add-user-modal .btn-primary');
    btn.textContent = 'Creating...';
    btn.disabled = true;

    const formData = new URLSearchParams();
    formData.append('name', name);
    formData.append('csrf_token', getCsrfToken());

    try {
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
        const modal = document.getElementById('api-key-modal');
        modal.style.display = 'flex';
        requestAnimationFrame(() => modal.classList.add('show'));
    } finally {
        btn.textContent = 'Create';
        btn.disabled = false;
    }
}

async function deleteUser(id, name) {
    if (!confirm('Delete user "' + name + '"? This will also delete all their usage data.')) return;

    const resp = await fetch('/admin/users/' + id, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': getCsrfToken() },
    });

    if (resp.ok) {
        const row = document.getElementById('user-row-' + id);
        row.style.opacity = '0';
        row.style.transform = 'translateX(-20px)';
        setTimeout(() => row.remove(), 300);
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

// Relative time for report page
document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('.time-ago').forEach(el => {
        const time = new Date(el.dataset.time);
        const diff = Math.floor((Date.now() - time) / 1000);
        if (diff < 60) el.title = diff + 's ago';
        else if (diff < 3600) el.title = Math.floor(diff / 60) + 'm ago';
        else if (diff < 86400) el.title = Math.floor(diff / 3600) + 'h ago';
        else el.title = Math.floor(diff / 86400) + 'd ago';
    });

    // Staggered row animation
    document.querySelectorAll('.data-table tbody tr').forEach((row, i) => {
        row.style.animationDelay = (i * 0.04) + 's';
    });
});
