// Phase 5: S3-based upload with presigned URLs
// No longer imports tus-js-client

// Environment flags
const IS_DEV = import.meta.env.DEV;

// Configuration
const CONFIG = {
  GOOGLE_CLIENT_ID: '784455620242-s0d334vgtbvoem5j3n0v0eo0lf03q2ua.apps.googleusercontent.com',
  // Backend URL - hardcoded for production, can be overridden for development
  BACKEND_URL: IS_DEV ? 'http://localhost:8080' : 'https://content.skatkis-tech.net',
};

const CREDIT_SCALE = 10000;

function formatCredits(units) {
  const num = Number(units || 0);
  return (num / CREDIT_SCALE).toFixed(4);
}

// DOM elements
const loginBtn = document.getElementById('loginBtn');
const logoutBtn = document.getElementById('logoutBtn');
const loginSection = document.getElementById('loginSection');
const userInfo = document.getElementById('userInfo');
const userEmail = document.getElementById('userEmail');
const uploadArea = document.getElementById('uploadArea');
const fileInput = document.getElementById('fileInput');
const fileInfo = document.getElementById('fileInfo');
const fileName = document.getElementById('fileName');
const fileSize = document.getElementById('fileSize');
const progressFill = document.getElementById('progressFill');
const progressText = document.getElementById('progressText');
const uploadBtn = document.getElementById('uploadBtn');
const cancelBtn = document.getElementById('cancelBtn');
const status = document.getElementById('status');

const MONETIZATION_REL = 'monetization';
const POINTER_PREFIX = 'https://';
const BALANCE_POLL_MS = 10000; // poll balance every 10s
const queueSection = document.getElementById('queueSection');
const queueList = document.getElementById('queueList');

let googleIdToken = null;
let selectedFile = null;
let currentUser = null;
let currentUploadAbortController = null;
let devUserEmail = localStorage.getItem('devUserEmail') || '';
let balancePollTimer = null;
let uploadQueue = [];
let isUploading = false;
let currentQueueItem = null;

function getAuthHeaders(extra = {}) {
  if (IS_DEV) {
    if (currentUser?.email) {
      return { ...extra, 'X-User-ID': currentUser.email };
    }
    return { ...extra };
  }
  if (googleIdToken) {
    return { ...extra, 'Authorization': `Bearer ${googleIdToken}` };
  }
  return { ...extra };
}

function hasAuth() {
  if (IS_DEV) {
    return Boolean(currentUser?.email);
  }
  return Boolean(googleIdToken);
}

// Handle Google Sign-In response
function handleCredentialResponse(response) {
  googleIdToken = response.credential;
  
  // Decode JWT to get user info
  const decoded = JSON.parse(atob(googleIdToken.split('.')[1]));
  currentUser = {
    email: decoded.email,
    id: decoded.sub,
  };
  
  // Save to localStorage
  localStorage.setItem('googleIdToken', googleIdToken);
  localStorage.setItem('currentUser', JSON.stringify(currentUser));
  
  updateUIAfterLogin();
}

// Initialize Google Sign-In
function initializeGoogleSignIn() {
  const script = document.createElement('script');
  script.src = 'https://accounts.google.com/gsi/client';
  script.async = true;
  script.defer = true;
  document.head.appendChild(script);

  window.addEventListener('load', function() {
    google.accounts.id.initialize({
      client_id: CONFIG.GOOGLE_CLIENT_ID,
      callback: handleCredentialResponse,
    });
    
    // Render the Sign-In button
    google.accounts.id.renderButton(
      loginBtn,
      { theme: 'outline', size: 'large' }
    );
  });
}

// Login button handler (for development with manual input)
if (IS_DEV) {
  loginBtn.addEventListener('click', (e) => {
    e.preventDefault();
    const email = prompt('Enter email for dev login');
    if (!email) {
      showStatus('Email required for dev login', 'error');
      return;
    }
    devUserEmail = email.trim();
    currentUser = { email: devUserEmail, id: devUserEmail };
    googleIdToken = 'dev-token';
    localStorage.setItem('devUserEmail', devUserEmail);
    localStorage.setItem('googleIdToken', googleIdToken);
    localStorage.setItem('currentUser', JSON.stringify(currentUser));
    updateUIAfterLogin();
  });
}

// Update UI after login
async function updateUIAfterLogin() {
  loginSection.style.display = 'none';
  userInfo.classList.add('show');
  userEmail.textContent = `${currentUser.email}`;
  uploadArea.style.display = 'block'; // Show upload area
  uploadArea.classList.remove('disabled');
  
  // Start Web Monetization
  ensureMonetization();
  
  // Verify authentication with server
  try {
    const response = await fetch(`${CONFIG.BACKEND_URL}/user`, {
      headers: getAuthHeaders(),
    });
    
    if (response.ok) {
      const data = await response.json();
      userEmail.textContent = `${data.email}`;
      const creditsEl = document.getElementById('userCredits');
      if (creditsEl && typeof data.credits !== 'undefined') {
        creditsEl.textContent = formatCredits(data.credits);
        creditsEl.classList.toggle('low', Number(data.credits) <= 0);
      }
      showStatus(`Logged in as ${data.email}`, 'success');
    } else {
      showStatus(`Logged in as ${currentUser.email}`, 'success');
    }
  } catch (err) {
    showStatus(`Logged in as ${currentUser.email}`, 'success');
  }
  
  setTimeout(clearStatus, 2000);

  // Load existing files after login
  loadFileList();

  // Begin periodic balance polling
  startBalancePolling();
}

// Logout
logoutBtn.addEventListener('click', () => {
  logoutUser();
});



function logoutUser() {
  googleIdToken = null;
  currentUser = null;
  loginSection.style.display = 'block';
  userInfo.classList.remove('show');
  uploadArea.style.display = 'none'; // Hide upload area completely
  uploadArea.classList.add('disabled');
  fileInfo.classList.remove('show');
  selectedFile = null;
  
  // Stop Web Monetization
  removeMonetization();

  stopBalancePolling();
  
  // Hide and clear file list and section
  const fileList = document.getElementById('filesList');
  if (fileList) fileList.innerHTML = '';
  const filesSection = document.getElementById('filesSection');
  if (filesSection) filesSection.style.display = 'none';
  
  // Clear localStorage
  localStorage.removeItem('googleIdToken');
  localStorage.removeItem('currentUser');
  localStorage.removeItem('devUserEmail');
  
  clearStatus();
  showStatus('Logged out', 'info');
  setTimeout(clearStatus, 2000);

  if (!IS_DEV && window.google && google.accounts?.id) {
    google.accounts.id.disableAutoSelect();
  }
}

// Upload area click
uploadArea.addEventListener('click', () => {
  if (!hasAuth()) {
    showStatus('Please sign in first', 'error');
    return;
  }
  fileInput.click();
});

// File input change
fileInput.addEventListener('change', (e) => {
  const file = e.target.files?.[0];
  if (file) {
    selectFile(file);
  }
});

// Drag and drop
uploadArea.addEventListener('dragover', (e) => {
  if (!hasAuth()) return;
  e.preventDefault();
  uploadArea.classList.add('dragover');
});

uploadArea.addEventListener('dragleave', () => {
  uploadArea.classList.remove('dragover');
});

uploadArea.addEventListener('drop', (e) => {
  if (!hasAuth()) return;
  e.preventDefault();
  uploadArea.classList.remove('dragover');
  const file = e.dataTransfer.files?.[0];
  if (file) {
    selectFile(file);
  }
});

// Select file
function selectFile(file) {
  selectedFile = file;
  fileName.textContent = file.name;
  fileSize.textContent = `${(file.size / 1024 / 1024).toFixed(2)} MB`;
  clearStatus();
  enqueueFile(file);
}

// Upload button triggers queue processing (uploads auto-start on selection)
uploadBtn.addEventListener('click', () => processQueue());

// Cancel button
cancelBtn.addEventListener('click', cancelUpload);

// Start upload - now using presigned URLs and a queue
async function startUpload(queueItem) {
  if (!hasAuth()) {
    showStatus('Please sign in first', 'error');
    return;
  }
  
  const file = queueItem?.file || selectedFile;
  if (!file) {
    showStatus('No file to upload', 'error');
    return;
  }

  selectedFile = file;
  fileName.textContent = file.name;
  fileSize.textContent = `${(file.size / 1024 / 1024).toFixed(2)} MB`;
  fileInfo.classList.add('show');
  uploadBtn.disabled = true;
  cancelBtn.disabled = false;

  showStatus('Requesting upload URL...', 'info');

  try {
    // Step 1: Get presigned URL from backend
    const uploadUrlResponse = await fetch(`${CONFIG.BACKEND_URL}/files/upload-url`, {
      method: 'POST',
      headers: {
        ...getAuthHeaders(),
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        filename: file.name,
        size: file.size,
        contentType: file.type,
      }),
    });

    if (!uploadUrlResponse.ok) {
      const errorData = await uploadUrlResponse.json().catch(() => ({}));
      throw new Error(errorData.error || `Server returned ${uploadUrlResponse.status}`);
    }

    const uploadData = await uploadUrlResponse.json();
    const { uploadId, cost, filename } = uploadData;

    showStatus(`Credits deducted: ${formatCredits(cost)}. Uploading file...`, 'info');
    progressFill.style.width = '10%';
    progressText.textContent = '10%';
    updateQueueItemStatus(queueItem.id, 'uploading');

    // Step 2: Upload file through backend proxy (avoids CORS issues)
    const xhr = new XMLHttpRequest();
    currentUploadAbortController = { abort: () => xhr.abort() };

    // Set timeout to 20 minutes for large files
    xhr.timeout = 1200000;

    // Real progress using XHR upload events
    xhr.upload.addEventListener('progress', (event) => {
      if (!event.lengthComputable) return;
      const ratio = event.loaded / event.total;
      const pct = Math.min(98, Math.max(10, ratio * 100));
      progressFill.style.width = pct + '%';
      progressText.textContent = Math.round(pct) + '%';
      console.log(`Upload progress: ${event.loaded}/${event.total} (${Math.round(ratio * 100)}%)`);
    });

    const uploadPromise = new Promise((resolve, reject) => {
      xhr.onreadystatechange = () => {
        console.log(`XHR state: ${xhr.readyState}, status: ${xhr.status}`);
        if (xhr.readyState !== XMLHttpRequest.DONE) return;
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(xhr.responseText);
        } else {
          reject(new Error(`Upload failed: ${xhr.status} ${xhr.statusText || ''}`.trim()));
        }
      };

      xhr.onerror = () => {
        console.error('XHR error event fired');
        reject(new Error('Network error during upload'));
      };
      xhr.onabort = () => {
        console.log('XHR abort event fired');
        reject(new DOMException('Aborted', 'AbortError'));
      };
      xhr.ontimeout = () => {
        console.error('XHR timeout event fired');
        reject(new Error('Upload timeout (20 minutes exceeded)'));
      };

      xhr.open('POST', `${CONFIG.BACKEND_URL}/files/${uploadId}/upload`);

      const headers = getAuthHeaders({ 'X-Filename': filename || file.name });
      Object.entries(headers).forEach(([k, v]) => {
        if (v) xhr.setRequestHeader(k, v);
      });
      if (file.type) {
        xhr.setRequestHeader('Content-Type', file.type);
      }

      console.log(`Starting XHR upload: ${file.size} bytes to ${CONFIG.BACKEND_URL}/files/${uploadId}/upload`);
      xhr.send(file);
    });

    const uploadResponseText = await uploadPromise;
    let uploadResult = {};
    try {
      uploadResult = JSON.parse(uploadResponseText || '{}');
    } catch (e) {
      uploadResult = {};
    }
    console.log('Upload result:', uploadResult);

    // Update progress
    progressFill.style.width = '98%';
    progressText.textContent = '98%';
    showStatus('Upload complete. Finalizing...', 'info');

    // Step 3: Mark upload as complete
    const completeResponse = await fetch(`${CONFIG.BACKEND_URL}/files/${uploadId}/complete`, {
      method: 'POST',
      headers: getAuthHeaders(),
    });

    if (!completeResponse.ok) {
      const errorData = await completeResponse.json().catch(() => ({}));
      throw new Error(errorData.error || 'Failed to mark upload complete');
    }

    // Success!
    showStatus(`✓ Upload completed: ${file.name}`, 'success');
    cancelBtn.disabled = true;
    progressFill.style.width = '100%';
    progressText.textContent = '100%';
    
    // Clear upload state
    localStorage.removeItem('uploadFileInfo');
    finishCurrentQueueItem(queueItem.id);

    // Refresh file list to show the new upload
    loadFileList();
    
    // Refresh credits after upload completes
    try {
      const resp = await fetch(`${CONFIG.BACKEND_URL}/user`, {
        headers: getAuthHeaders(),
      });
      if (resp.ok) {
        const data = await resp.json();
        const creditsEl = document.getElementById('userCredits');
        if (creditsEl && typeof data.credits !== 'undefined') {
          creditsEl.textContent = formatCredits(data.credits);
          creditsEl.classList.toggle('low', Number(data.credits) <= 0);
        }
      }
    } catch (err) {
      console.error('Failed to refresh credits:', err);
    }
    
    // Reset after 1 second and move to next
    setTimeout(() => {
      selectedFile = null;
      fileInfo.classList.remove('show');
      progressFill.style.width = '0%';
      progressText.textContent = '0%';
      clearStatus();
    }, 1000);
  } catch (error) {
    console.error('Upload error:', error);
    
    if (error.name === 'AbortError') {
      showStatus('Upload cancelled', 'info');
    } else {
      showStatus(`Upload error: ${error.message}`, 'error');
    }
    
    // Reset UI
    uploadBtn.disabled = false;
    cancelBtn.disabled = true;
    progressFill.style.width = '0%';
    progressText.textContent = '0%';
    finishCurrentQueueItem(queueItem.id);
    
    setTimeout(clearStatus, 3000);
  } finally {
    currentUploadAbortController = null;
  }
}

// Cancel upload
function cancelUpload() {
  if (currentUploadAbortController) {
    currentUploadAbortController.abort();
    currentUploadAbortController = null;
  }
  if (currentQueueItem) {
    finishCurrentQueueItem(currentQueueItem.id);
  }
  isUploading = false;
  currentQueueItem = null;
  processQueue();
}

function enqueueFile(file) {
  const id = (crypto.randomUUID ? crypto.randomUUID() : `q_${Date.now()}_${Math.random().toString(16).slice(2)}`);
  uploadQueue.push({ id, file, status: 'queued' });
  renderQueue();
  processQueue();
}

function processQueue() {
  if (isUploading) return;
  const next = uploadQueue.find(item => item.status === 'queued');
  if (!next) {
    renderQueue();
    return;
  }
  isUploading = true;
  currentQueueItem = next;
  updateQueueItemStatus(next.id, 'uploading');
  startUpload(next).finally(() => {
    isUploading = false;
    currentQueueItem = null;
    processQueue();
  });
}

function finishCurrentQueueItem(id) {
  uploadQueue = uploadQueue.filter(item => item.id !== id);
  renderQueue();
}

function updateQueueItemStatus(id, status) {
  const item = uploadQueue.find(i => i.id === id);
  if (item) {
    item.status = status;
  }
  renderQueue();
}

function renderQueue() {
  if (!queueSection || !queueList) return;
  if (!uploadQueue.length) {
    queueSection.style.display = 'none';
    queueList.innerHTML = '';
    cancelBtn.disabled = true;
    uploadBtn.disabled = false;
    return;
  }
  const uploading = uploadQueue.some(item => item.status === 'uploading');
  queueSection.style.display = 'block';
  cancelBtn.disabled = !uploading;
  uploadBtn.disabled = uploading;
  queueList.innerHTML = uploadQueue.map((item, idx) => {
    const statusClass = item.status === 'uploading' ? 'uploading' : 'queued';
    const label = item.status === 'uploading' ? 'Uploading' : 'Queued';
    const prefix = idx === 0 && item.status === 'uploading' ? '▶ ' : '';
    return `
      <div class="queue-item">
        <div class="name">${prefix}${escapeHtml(item.file.name)}</div>
        <div class="meta">${(item.file.size / 1024 / 1024).toFixed(2)} MB</div>
        <div class="status-pill ${statusClass}">${label}</div>
      </div>
    `;
  }).join('');
}

// Show/clear status
function showStatus(message, type = 'info') {
  status.textContent = message;
  status.className = `status show ${type}`;
}

function clearStatus() {
  status.textContent = '';
  status.className = 'status';
}

// Restore session from localStorage
function restoreSession() {
  const savedToken = localStorage.getItem('googleIdToken');
  const savedUser = localStorage.getItem('currentUser');
  
  if (savedToken && savedUser) {
    try {
      googleIdToken = savedToken;
      currentUser = JSON.parse(savedUser);
      updateUIAfterLogin();
    } catch (err) {
      console.error('Failed to restore session:', err);
      localStorage.removeItem('googleIdToken');
      localStorage.removeItem('currentUser');
    }
  }
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
  // Initialize Google Sign-In only for prod
  if (!IS_DEV) {
    initializeGoogleSignIn();
  }
  
  // Hide upload area initially (will be shown after login)
  uploadArea.style.display = 'none';
  
  // Restore previous session if available
  restoreSession();

  // Web Monetization will be enabled after login
  // Do not enable it here when logged out
});

// Ensure a spec-compliant <link rel="monetization"> exists; attach listeners
async function ensureMonetization() {
  try {
    // If link already present with href, keep it
    let link = document.querySelector(`link[rel="${MONETIZATION_REL}"]`);
    if (link && link.getAttribute('href')) {
      normalizePaymentPointerOnLink(link);
      attachMonetizationListeners();
      return;
    }

    const resp = await fetch(`${CONFIG.BACKEND_URL}/monetization/config`);
    if (!resp.ok) return;
    const cfg = await resp.json();
    if (!cfg.enabled || !cfg.payment_pointer) return;

    // Remove any existing monetization elements to avoid duplicates
    document.querySelectorAll(`link[rel="${MONETIZATION_REL}"]`).forEach(n => n.remove());

    link = document.createElement('link');
    link.setAttribute('rel', MONETIZATION_REL);
    link.setAttribute('href', normalizePaymentPointer(cfg.payment_pointer));
    document.head.appendChild(link);

    attachMonetizationListeners();
  } catch (err) {
    console.warn('Monetization config fetch failed:', err);
  }
}

function normalizePaymentPointerOnLink(link) {
  const href = link.getAttribute('href') || '';
  const norm = normalizePaymentPointer(href);
  if (href !== norm) {
    link.setAttribute('href', norm);
  }
}

// Convert $wallet-address to https://wallet-address to avoid relative URL resolution
function normalizePaymentPointer(pointer) {
  if (!pointer) return '';
  const trimmed = pointer.trim();
  if (trimmed.startsWith('$')) {
    return POINTER_PREFIX + trimmed.slice(1);
  }
  if (trimmed.startsWith('https://') || trimmed.startsWith('http://')) {
    return trimmed;
  }
  // Fallback: treat as host/path
  return POINTER_PREFIX + trimmed;
}

function attachMonetizationListeners() {
  const link = document.querySelector(`link[rel="${MONETIZATION_REL}"]`);
  if (link) {
    link.addEventListener('monetization', (event) => {
      const { amountSent, paymentPointer, incomingPayment } = event;
      console.debug('[WM] monetization event', { amountSent, paymentPointer, incomingPayment });
      if (incomingPayment && googleIdToken) {
        postMonetizationIncomingPayment(incomingPayment).catch(() => {});
      }
    });
  }
}

function removeMonetization() {
  // Remove all monetization links from the page
  document.querySelectorAll(`link[rel="${MONETIZATION_REL}"]`).forEach(n => n.remove());
  console.debug('[WM] Removed monetization links');
}

async function postMonetizationIncomingPayment(incomingPaymentUrl) {
  const body = { receipt_url: incomingPaymentUrl };
  console.debug('[WM] verify start', body);
  const resp = await fetch(`${CONFIG.BACKEND_URL}/monetization/verify`, {
    method: 'POST',
    headers: getAuthHeaders({ 'Content-Type': 'application/json' }),
    body: JSON.stringify(body),
  });

  if (resp.ok) {
    const data = await resp.json();
    console.debug('[WM] verify ok', data);
    const creditsEl = document.getElementById('userCredits');
    if (creditsEl && typeof data.credits !== 'undefined') {
      creditsEl.textContent = formatCredits(data.credits);
      creditsEl.classList.toggle('low', Number(data.credits) <= 0);
    }
  } else {
    const text = await resp.text();
    console.warn('[WM] verify failed', resp.status, text);
  }
}

// File Browser Functions

async function loadFileList() {
  try {
    console.log('[FileList] Loading files from', `${CONFIG.BACKEND_URL}/files`);
    const resp = await fetch(`${CONFIG.BACKEND_URL}/files`, {
      headers: getAuthHeaders(),
    });

    console.log('[FileList] Response status:', resp.status);
    if (!resp.ok) {
      const errorText = await resp.text();
      console.error('Failed to load files:', resp.status, errorText);
      return;
    }

    const files = await resp.json();
    console.log('[FileList] Loaded files:', files);
    displayFileList(files);
  } catch (err) {
    console.error('Error loading files:', err);
  }
}

function displayFileList(files) {
  const filesList = document.getElementById('filesList');
  const filesSection = document.getElementById('filesSection');

  if (!files || files.length === 0) {
    filesList.innerHTML = '<div class="empty-state">No files uploaded yet</div>';
    filesSection.style.display = 'block';
    return;
  }

  filesSection.style.display = 'block';
  filesList.innerHTML = files.map(file => `
    <div class="file-item">
      <div class="file-item-name">${escapeHtml(file.filename)}</div>
      <div class="file-item-meta">
        <div>${formatBytes(file.size)}</div>
        <div>Upload charged: ${formatCredits(file.uploadCost)} credits</div>
        <div>Download cost: ${formatCredits(file.downloadCost)} credits</div>
        <div>Storage/month: ${formatCredits(file.storageMonthlyCost)} credits</div>
        <div>${new Date(file.uploadedAt).toLocaleDateString()}</div>
      </div>
      <div class="file-item-actions">
        <button class="file-item-btn file-item-download" onclick="downloadFile('${file.id}', '${escapeHtml(file.filename)}')">
          Download
        </button>
        <button class="file-item-btn file-item-delete" onclick="deleteFile('${file.id}', '${escapeHtml(file.filename)}')">
          Delete
        </button>
        <button class="file-item-btn" style="background:#e0eaff; color:#333;" onclick="shareFile('${file.id}', '${escapeHtml(file.filename)}')">
          Share
        </button>
      </div>
    </div>
  `).join('');
}

window.shareFile = async function(uploadID, filename) {
  if (!hasAuth()) {
    showStatus('Please sign in first', 'error');
    return;
  }
  try {
    const resp = await fetch(`${CONFIG.BACKEND_URL}/files/${uploadID}/share`, {
      method: 'POST',
      headers: getAuthHeaders(),
    });
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}));
      showStatus('Failed to create share link: ' + (err.error || resp.status), 'error');
      return;
    }
    const data = await resp.json();
    showShareModal(data.url, data.expires_at);
  } catch (err) {
    showStatus('Failed to create share link: ' + err.message, 'error');
  }
};

function showShareModal(url, expiresAt) {
  const modal = document.getElementById('shareModal');
  const input = document.getElementById('shareLinkInput');
  const expiry = document.getElementById('shareExpiry');
  input.value = url;
  expiry.textContent = expiresAt ? `Expires: ${new Date(expiresAt).toLocaleString()}` : '';
  modal.style.display = 'flex';
}

document.addEventListener('DOMContentLoaded', () => {
  const modal = document.getElementById('shareModal');
  const closeBtn = document.getElementById('closeShareModal');
  const copyBtn = document.getElementById('copyShareLinkBtn');
  const input = document.getElementById('shareLinkInput');
  if (closeBtn) closeBtn.onclick = () => { modal.style.display = 'none'; };
  if (modal) modal.onclick = (e) => { if (e.target === modal) modal.style.display = 'none'; };
  if (copyBtn) copyBtn.onclick = () => {
    input.select();
    document.execCommand('copy');
    copyBtn.textContent = 'Copied!';
    setTimeout(() => { copyBtn.textContent = 'Copy Link'; }, 1200);
  };
});

window.downloadFile = async function(uploadID, filename) {
  if (!confirm(`Download ${filename}?`)) {
    return;
  }

  try {
    const resp = await fetch(`${CONFIG.BACKEND_URL}/files/${uploadID}/download`, {
      headers: getAuthHeaders(),
    });

    if (resp.status === 402) {
      showStatus('Account frozen or insufficient credits', 'error');
      return;
    }

    if (!resp.ok) {
      showStatus(`Download failed: ${resp.status}`, 'error');
      return;
    }

    // Create blob and download
    const blob = await resp.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);

    showStatus('File downloaded successfully', 'success');
    refreshBalance();
  } catch (err) {
    console.error('Download error:', err);
    showStatus('Download failed: ' + err.message, 'error');
  }
};

window.deleteFile = async function(uploadID, filename) {
  if (!confirm(`Delete ${filename}? This cannot be undone.`)) {
    return;
  }

  try {
    const resp = await fetch(`${CONFIG.BACKEND_URL}/files/${uploadID}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    });

    if (!resp.ok) {
      showStatus(`Delete failed: ${resp.status}`, 'error');
      return;
    }

    showStatus('File deleted successfully', 'success');
    loadFileList();
  } catch (err) {
    console.error('Delete error:', err);
    showStatus('Delete failed: ' + err.message, 'error');
  }
};

function escapeHtml(text) {
  if (!text) return '';
  const map = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;',
  };
  return text.replace(/[&<>"']/g, m => map[m]);
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

async function refreshBalance() {
  try {
    const resp = await fetch(`${CONFIG.BACKEND_URL}/user`, { headers: getAuthHeaders() });
    if (!resp.ok) return;
    const data = await resp.json();
    const creditsEl = document.getElementById('userCredits');
    if (creditsEl && typeof data.credits !== 'undefined') {
      creditsEl.textContent = formatCredits(data.credits);
      creditsEl.classList.toggle('low', Number(data.credits) <= 0);
    }
  } catch {}
}

function startBalancePolling() {
  stopBalancePolling();
  if (!hasAuth()) return;
  balancePollTimer = setInterval(() => {
    if (!hasAuth()) {
      stopBalancePolling();
      return;
    }
    refreshBalance();
  }, BALANCE_POLL_MS);
  // Prime immediately so UI updates quickly
  refreshBalance();
}

function stopBalancePolling() {
  if (balancePollTimer) {
    clearInterval(balancePollTimer);
    balancePollTimer = null;
  }
}

// Setup file browser refresh button
document.getElementById('refreshFilesBtn').addEventListener('click', loadFileList);


