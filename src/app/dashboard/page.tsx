"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Download, File, FileText, HardDrive, Image as ImageIcon, Music, Film, Share, Trash2, Upload, Vault, Wallet, X } from "lucide-react";

import { authHeaders, BACKEND_URL, BALANCE_POLL_MS, formatCredits, GOOGLE_CLIENT_ID } from "@/lib/backend";

type ServerUser = {
  email: string;
  id: string;
  credits: number;
};

type ServerFile = {
  id: string;
  uploadId: string;
  filename: string;
  size: number;
  uploadedAt: string;
  uploadCost: number;
  downloadCost: number;
  storageMonthlyCost: number;
};

const formatBytes = (bytes: number): string => {
  if (!bytes) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1);
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
};

const fileTypeLabel = (name: string) => {
  const lowered = name.toLowerCase();
  if (lowered.match(/\.(png|jpe?g|gif|webp|svg)$/)) return "Image";
  if (lowered.match(/\.(mp4|mov|mkv|webm)$/)) return "Video";
  if (lowered.match(/\.(mp3|wav|flac)$/)) return "Audio";
  if (lowered.match(/\.(zip|tar|gz)$/)) return "Archive";
  if (lowered.match(/\.(pdf)$/)) return "PDF";
  return "File";
};

const fileIcon = (name: string) => {
  const lowered = name.toLowerCase();
  if (lowered.match(/\.(png|jpe?g|gif|webp|svg)$/)) return ImageIcon;
  if (lowered.match(/\.(mp4|mov|mkv|webm)$/)) return Film;
  if (lowered.match(/\.(mp3|wav|flac)$/)) return Music;
  if (lowered.match(/\.(zip|tar|gz)$/)) return FileText;
  if (lowered.match(/\.(pdf)$/)) return FileText;
  return File;
};

const normalizePaymentPointer = (pointer: string) => {
  if (!pointer) return "";
  const trimmed = pointer.trim();
  if (trimmed.startsWith("$")) return `https://${trimmed.slice(1)}`;
  if (trimmed.startsWith("http")) return trimmed;
  return `https://${trimmed}`;
};

export default function Dashboard() {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<ServerUser | null>(null);
  const [files, setFiles] = useState<ServerFile[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [queue, setQueue] = useState<File[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [currentFileName, setCurrentFileName] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [shareLink, setShareLink] = useState<string | null>(null);
  const [shareExpiry, setShareExpiry] = useState<string | null>(null);
  const loginButtonRef = useRef<HTMLDivElement>(null);
  const balanceTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // Boot Google Sign-In once on mount
  useEffect(() => {
    const stored = localStorage.getItem("googleIdToken");
    if (stored) {
      setToken(stored);
    }

    const script = document.createElement("script");
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.defer = true;
    script.onload = () => {
      if (!(window as any).google || !GOOGLE_CLIENT_ID || !loginButtonRef.current) return;
      (window as any).google.accounts.id.initialize({
        client_id: GOOGLE_CLIENT_ID,
        callback: (response: any) => {
          if (response?.credential) {
            setToken(response.credential);
            localStorage.setItem("googleIdToken", response.credential);
          }
        },
      });
      (window as any).google.accounts.id.renderButton(loginButtonRef.current, {
        theme: "outline",
        size: "large",
      });
    };
    document.head.appendChild(script);
  }, []);

  // Load user/files and start balance polling once authenticated
  useEffect(() => {
    if (!token) {
      setUser(null);
      setFiles([]);
      if (balanceTimer.current) clearInterval(balanceTimer.current);
      return;
    }

    const init = async () => {
      await Promise.all([refreshUser(token), refreshFiles(token), ensureMonetization(token)]);
    };
    init().catch((err) => console.error("init error", err));

    balanceTimer.current = setInterval(() => {
      refreshUser(token).catch(() => {});
    }, BALANCE_POLL_MS);

    return () => {
      if (balanceTimer.current) clearInterval(balanceTimer.current);
    };
  }, [token]);

  const refreshUser = async (idToken: string) => {
    const res = await fetch(`${BACKEND_URL}/user`, {
      headers: authHeaders(idToken),
    });
    if (!res.ok) return;
    const data = (await res.json()) as ServerUser;
    setUser(data);
  };

  const refreshFiles = async (idToken: string) => {
    const res = await fetch(`${BACKEND_URL}/files`, {
      headers: authHeaders(idToken),
    });
    if (!res.ok) return;
    const data = (await res.json()) as ServerFile[];
    setFiles(data);
  };

  const ensureMonetization = async (idToken: string) => {
    try {
      const res = await fetch(`${BACKEND_URL}/monetization/config`);
      if (!res.ok) return;
      const cfg = await res.json();
      if (!cfg.enabled || !cfg.payment_pointer) return;

      // remove existing monetization links to avoid duplicates
      document
        .querySelectorAll('link[rel="monetization"]')
        .forEach((n) => n.remove());

      const link = document.createElement("link");
      link.setAttribute("rel", "monetization");
      link.setAttribute("href", normalizePaymentPointer(cfg.payment_pointer));
      document.head.appendChild(link);

      const handler = async (event: any) => {
        if (!event?.detail?.incomingPayment) return;
        try {
          await fetch(`${BACKEND_URL}/monetization/verify`, {
            method: "POST",
            headers: authHeaders(idToken, { "Content-Type": "application/json" }),
            body: JSON.stringify({ receipt_url: event.detail.incomingPayment }),
          });
          refreshUser(idToken);
        } catch (err) {
          console.warn("monetization verify failed", err);
        }
      };

      document.addEventListener("monetizationprogress", handler as any);
      return () => document.removeEventListener("monetizationprogress", handler as any);
    } catch (err) {
      console.warn("monetization config fetch failed", err);
    }
    return undefined;
  };

  const handleLogout = () => {
    localStorage.removeItem("googleIdToken");
    setToken(null);
    setUser(null);
    setFiles([]);
    setStatus(null);
  };

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback(
    async (e: React.DragEvent) => {
      e.preventDefault();
      setIsDragging(false);
      if (!token) return;
      const dropped = Array.from(e.dataTransfer.files || []);
      if (dropped.length) {
        // enqueue all files
        setQueue((prev) => [...prev, ...dropped]);
      }
    },
    [token]
  );

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!token) return;
    const selected = e.target.files ? Array.from(e.target.files) : [];
    if (selected.length) {
      setQueue((prev) => [...prev, ...selected]);
    }
  };

  // Process queue sequentially whenever there are items and not currently uploading
  useEffect(() => {
    if (!token) return;
    if (isUploading) return;
    if (queue.length === 0) return;

    const current = queue[0];
    setIsUploading(true);
    setCurrentFileName(current.name);
    uploadFile(current, token)
      .catch(() => {})
      .finally(() => {
        setQueue((prev) => prev.slice(1));
        setIsUploading(false);
        setCurrentFileName(null);
        abortRef.current = null;
      });
  }, [queue, isUploading, token]);

  const uploadFile = async (file: File, idToken: string) => {
    setUploadProgress(5);
    setStatus("Requesting upload slot...");
    try {
      const urlRes = await fetch(`${BACKEND_URL}/files/upload-url`, {
        method: "POST",
        headers: authHeaders(idToken, { "Content-Type": "application/json" }),
        body: JSON.stringify({ filename: file.name, size: file.size, contentType: file.type || "application/octet-stream" }),
      });

      if (urlRes.status === 402) {
        setStatus("Account frozen or insufficient credits to upload.");
        setUploadProgress(null);
        return;
      }

      if (!urlRes.ok) {
        const msg = await urlRes.text();
        throw new Error(msg || "Failed to request upload URL");
      }

      const { uploadId } = await urlRes.json();
      setUploadProgress(25);
      setStatus("Uploading via proxy...");

      const form = new FormData();
      form.append("file", file);
      const ac = new AbortController();
      abortRef.current = ac;
      const proxyRes = await fetch(`${BACKEND_URL}/files/${uploadId}/upload`, {
        method: "POST",
        headers: authHeaders(idToken, { "X-Filename": file.name }),
        body: form,
        signal: ac.signal,
      });

      if (!proxyRes.ok) {
        const msg = await proxyRes.text();
        throw new Error(msg || "Proxy upload failed");
      }

      setUploadProgress(90);
      setStatus("Finalizing upload...");

      const completeRes = await fetch(`${BACKEND_URL}/files/${uploadId}/complete`, {
        method: "POST",
        headers: authHeaders(idToken, { "Content-Type": "application/json" }),
      });

      if (!completeRes.ok) {
        const msg = await completeRes.text();
        throw new Error(msg || "Failed to finalize upload");
      }

      setUploadProgress(100);
      setStatus("Upload complete");
      await Promise.all([refreshFiles(idToken), refreshUser(idToken)]);
    } catch (err: any) {
      console.error("upload error", err);
      setStatus(err?.message || "Upload failed");
    } finally {
      setTimeout(() => setUploadProgress(null), 1200);
      setTimeout(() => setStatus(null), 3000);
    }
  };

  const cancelCurrentUpload = () => {
    if (abortRef.current) {
      abortRef.current.abort();
      setStatus("Upload canceled. Note: ingress cost may already be charged.");
      setUploadProgress(null);
      setIsUploading(false);
      setCurrentFileName(null);
      // Drop current item from queue
      setQueue((prev) => prev.slice(1));
      abortRef.current = null;
    }
  };

  const removeQueuedItem = (idx: number) => {
    setQueue((prev) => prev.filter((_, i) => i !== idx));
  };

  const handleDownload = async (file: ServerFile) => {
    if (!token) return;
    const res = await fetch(`${BACKEND_URL}/files/${file.id}/download`, {
      headers: authHeaders(token),
    });

    if (res.status === 402) {
      setStatus("Account frozen or insufficient credits to download.");
      return;
    }
    if (!res.ok) {
      setStatus("Download failed");
      return;
    }

    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = file.filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    setStatus("Downloaded");
    refreshUser(token);
  };

  const handleDelete = async (file: ServerFile) => {
    if (!token) return;
    if (!confirm(`Delete ${file.filename}? This cannot be undone.`)) return;
    const res = await fetch(`${BACKEND_URL}/files/${file.id}`, {
      method: "DELETE",
      headers: authHeaders(token),
    });
    if (!res.ok) {
      setStatus("Delete failed");
      return;
    }
    setFiles((prev) => prev.filter((f) => f.id !== file.id));
    setStatus("File deleted");
  };

  const handleShare = async (file: ServerFile) => {
    if (!token) return;
    const res = await fetch(`${BACKEND_URL}/files/${file.id}/share`, {
      method: "POST",
      headers: authHeaders(token),
    });
    if (!res.ok) {
      setStatus("Failed to create share link");
      return;
    }
    const data = await res.json();
    setShareLink(data.url);
    setShareExpiry(data.expires_at || null);
  };

  const totalStorage = files.reduce((acc, f) => acc + f.size, 0);
  const totalStorageMonthly = files.reduce((acc, f) => acc + f.storageMonthlyCost, 0);

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-100">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-emerald-400 to-emerald-600 rounded-lg flex items-center justify-center">
              <Vault className="w-5 h-5 text-white" />
            </div>
            <span className="text-xl font-semibold text-gray-900">MicroVault</span>
          </Link>

          <div className="flex items-center gap-4">
            {token ? (
              <>
                <div className="text-right">
                  <p className="text-sm text-gray-500">Credits</p>
                  <p className="text-lg font-semibold text-gray-900">{formatCredits(user?.credits)}</p>
                </div>
                <button
                  onClick={handleLogout}
                  className="px-4 py-2 bg-gray-100 text-gray-700 rounded-full hover:bg-gray-200 transition-colors"
                >
                  Log out
                </button>
              </>
            ) : (
              <div ref={loginButtonRef} className="flex items-center" />
            )}
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8 space-y-8">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="bg-white rounded-2xl p-6 border border-gray-100">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-emerald-50 rounded-xl flex items-center justify-center">
                <HardDrive className="w-6 h-6 text-emerald-600" />
              </div>
              <div>
                <p className="text-sm text-gray-500">Storage Used</p>
                <p className="text-2xl font-semibold text-gray-900">{formatBytes(totalStorage)}</p>
              </div>
            </div>
          </div>
          <div className="bg-white rounded-2xl p-6 border border-gray-100">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-amber-50 rounded-xl flex items-center justify-center">
                <Wallet className="w-6 h-6 text-amber-600" />
              </div>
              <div>
                <p className="text-sm text-gray-500">Balance (credits)</p>
                <p className={`text-2xl font-semibold ${Number(user?.credits || 0) <= 0 ? "text-red-600" : "text-gray-900"}`}>
                  {formatCredits(user?.credits)}
                </p>
              </div>
            </div>
          </div>
          <div className="bg-white rounded-2xl p-6 border border-gray-100">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-violet-50 rounded-xl flex items-center justify-center">
                <Upload className="w-6 h-6 text-violet-600" />
              </div>
              <div>
                <p className="text-sm text-gray-500">Storage/month</p>
                <p className="text-2xl font-semibold text-gray-900">{formatCredits(totalStorageMonthly)} credits</p>
              </div>
            </div>
          </div>
        </div>

        <div
          className={`relative border-2 border-dashed rounded-2xl p-12 text-center transition-all duration-200 ${
            isDragging ? "border-emerald-500 bg-emerald-50" : "border-gray-200 bg-white hover:border-gray-300"
          } ${!token ? "opacity-60 pointer-events-none" : ""}`}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
        >
          <input
            type="file"
            multiple
            onChange={handleFileSelect}
            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
            disabled={!token}
          />
          <div className="w-16 h-16 bg-gray-100 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <Upload className={`w-8 h-8 ${isDragging ? "text-emerald-600" : "text-gray-400"}`} />
          </div>
          <p className="text-lg font-medium text-gray-900 mb-2">
            {token ? (isDragging ? "Drop files here" : "Drag and drop files here") : "Sign in to start uploading"}
          </p>
          <p className="text-gray-500">or click to browse • uploads are charged upfront</p>
          {uploadProgress !== null && (
            <div className="mt-6 max-w-xs mx-auto">
              <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
                <div className="h-full bg-emerald-500 transition-all duration-300" style={{ width: `${uploadProgress}%` }} />
              </div>
              <p className="text-sm text-gray-500 mt-2">{status || "Uploading..."}</p>
            </div>
          )}

          {token && queue.length > 0 && (
            <div className="mt-4 text-sm text-gray-600">
              Queue: {queue.length} item{queue.length > 1 ? "s" : ""} pending
            </div>
          )}
        </div>

        <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-900">Your Files</h2>
            <button
              onClick={() => token && refreshFiles(token)}
              className="text-sm text-gray-500 hover:text-gray-800"
            >
              Refresh
            </button>
          </div>

          {files.length === 0 ? (
            <div className="px-6 py-16 text-center">
              <div className="w-16 h-16 bg-gray-100 rounded-2xl flex items-center justify-center mx-auto mb-4">
                <File className="w-8 h-8 text-gray-300" />
              </div>
              <p className="text-gray-500">No files uploaded yet</p>
              <p className="text-sm text-gray-400 mt-1">Upload your first file to get started</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">File</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Size</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Uploaded</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Ingress</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Egress</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Storage/mo</th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {files.map((file) => {
                    const Icon = fileIcon(file.filename);
                    return (
                      <tr key={file.id} className="hover:bg-gray-50 transition-colors">
                        <td className="px-6 py-4">
                          <div className="flex items-center gap-3">
                            <div className="w-10 h-10 bg-gray-100 rounded-lg flex items-center justify-center">
                              <Icon className="w-5 h-5 text-gray-500" />
                            </div>
                            <div className="max-w-xs">
                              <p className="font-medium text-gray-900 truncate">{file.filename}</p>
                              <p className="text-xs text-gray-500 truncate">{file.uploadId}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-gray-600">{formatBytes(file.size)}</td>
                        <td className="px-6 py-4 text-gray-600">{new Date(file.uploadedAt).toLocaleString()}</td>
                        <td className="px-6 py-4 text-gray-700">{formatCredits(file.uploadCost)}</td>
                        <td className="px-6 py-4 text-gray-700">{formatCredits(file.downloadCost)}</td>
                        <td className="px-6 py-4 text-gray-700">{formatCredits(file.storageMonthlyCost)}</td>
                        <td className="px-6 py-4">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => handleDownload(file)}
                              className="p-2 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors"
                              title="Download"
                            >
                              <Download className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => handleShare(file)}
                              className="p-2 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors"
                              title="Share"
                            >
                              <Share className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => handleDelete(file)}
                              className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                              title="Delete"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Upload Queue */}
        {token && (queue.length > 0 || isUploading) && (
          <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-100 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-gray-900">Upload Queue</h2>
              <span className="text-sm text-gray-500">{queue.length} pending</span>
            </div>
            <div className="divide-y divide-gray-50">
              {isUploading && currentFileName && (
                <div className="px-6 py-4 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 bg-emerald-50 rounded-lg flex items-center justify-center">
                      <Upload className="w-5 h-5 text-emerald-600" />
                    </div>
                    <div>
                      <p className="font-medium text-gray-900">{currentFileName}</p>
                      <p className="text-xs text-gray-500">Uploading...</p>
                    </div>
                  </div>
                  <button
                    onClick={cancelCurrentUpload}
                    className="p-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                    title="Cancel upload"
                  >
                    <X className="w-4 h-4" />
                  </button>
                </div>
              )}

              {queue.map((f, idx) => (
                <div key={`${f.name}-${idx}`} className="px-6 py-4 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 bg-gray-100 rounded-lg flex items-center justify-center">
                      <File className="w-5 h-5 text-gray-500" />
                    </div>
                    <div>
                      <p className="font-medium text-gray-900">{f.name}</p>
                      <p className="text-xs text-gray-500">Queued</p>
                    </div>
                  </div>
                  <button
                    onClick={() => removeQueuedItem(idx)}
                    className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                    title="Remove from queue"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}
      </main>

      {status && (
        <div className="fixed bottom-4 right-4 bg-gray-900 text-white px-4 py-3 rounded-lg shadow-lg text-sm">
          {status}
        </div>
      )}

      {shareLink && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl max-w-lg w-full p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-xl font-semibold text-gray-900">Share Link</h3>
              <button onClick={() => setShareLink(null)} className="text-gray-500 hover:text-gray-800">×</button>
            </div>
            <p className="text-sm text-gray-600">Anyone with this link can download. Egress is billed to you.</p>
            <div className="flex items-center gap-2">
              <input
                value={shareLink}
                readOnly
                className="flex-1 px-3 py-2 border border-gray-200 rounded-lg text-sm"
                onFocus={(e) => e.target.select()}
              />
              <button
                onClick={() => {
                  navigator.clipboard.writeText(shareLink);
                }}
                className="px-3 py-2 bg-gray-900 text-white rounded-lg text-sm"
              >
                Copy
              </button>
            </div>
            {shareExpiry && <p className="text-xs text-gray-500">Expires: {new Date(shareExpiry).toLocaleString()}</p>}
          </div>
        </div>
      )}
    </div>
  );
}

