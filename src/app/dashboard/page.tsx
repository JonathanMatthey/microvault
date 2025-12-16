"use client";

import { useState, useCallback, useEffect } from "react";
import Link from "next/link";
import { 
  Vault, 
  Upload, 
  Trash2, 
  Download, 
  FileText, 
  Image as ImageIcon, 
  Film, 
  Music,
  Archive,
  File,
  Plus,
  Wallet,
  Clock,
  HardDrive,
  AlertCircle,
  X,
  Check
} from "lucide-react";

type FileData = {
  id: string;
  name: string;
  size: number;
  mimeType: string;
  uploadedAt: string;
  costPerHour: number;
  totalCost: number;
  isLocked: boolean;
};

type UserData = {
  id: string;
  email: string;
  name: string;
  balance: number;
  walletAddress: string | null;
};

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

const formatCurrency = (cents: number): string => {
  if (cents < 1) {
    return `$${cents.toFixed(6)}`;
  }
  return `$${(cents / 100).toFixed(2)}`;
};

const getFileIcon = (mimeType: string) => {
  if (mimeType.startsWith("image/")) return ImageIcon;
  if (mimeType.startsWith("video/")) return Film;
  if (mimeType.startsWith("audio/")) return Music;
  if (mimeType.includes("zip") || mimeType.includes("archive") || mimeType.includes("compressed")) return Archive;
  if (mimeType.includes("pdf") || mimeType.includes("document") || mimeType.includes("text")) return FileText;
  return File;
};

const getFileTypeLabel = (mimeType: string): string => {
  if (mimeType.startsWith("image/")) return "Image";
  if (mimeType.startsWith("video/")) return "Video";
  if (mimeType.startsWith("audio/")) return "Audio";
  if (mimeType.includes("zip") || mimeType.includes("archive")) return "Archive";
  if (mimeType.includes("pdf")) return "PDF";
  if (mimeType.includes("document") || mimeType.includes("text")) return "Document";
  return "File";
};

// Pricing: $0.001 per GB per hour
const PRICE_PER_GB_PER_HOUR = 0.001;

const calculateCostPerHour = (sizeBytes: number): number => {
  const sizeGB = sizeBytes / (1024 * 1024 * 1024);
  return sizeGB * PRICE_PER_GB_PER_HOUR;
};

export default function Dashboard() {
  const [files, setFiles] = useState<FileData[]>([]);
  const [user, setUser] = useState<UserData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [showTopUp, setShowTopUp] = useState(false);
  const [topUpAmount, setTopUpAmount] = useState("1.00");
  const [walletAddress, setWalletAddress] = useState("");

  // Fetch user and files
  useEffect(() => {
    const fetchData = async () => {
      try {
        const [userRes, filesRes] = await Promise.all([
          fetch("/api/user"),
          fetch("/api/files"),
        ]);
        
        if (userRes.ok) {
          const userData = await userRes.json();
          setUser(userData);
        }
        
        if (filesRes.ok) {
          const filesData = await filesRes.json();
          setFiles(filesData);
        }
      } catch (error) {
        console.error("Error fetching data:", error);
      } finally {
        setIsLoading(false);
      }
    };
    
    fetchData();
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    
    const droppedFiles = Array.from(e.dataTransfer.files);
    if (droppedFiles.length > 0) {
      await uploadFiles(droppedFiles);
    }
  }, []);

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = e.target.files;
    if (selectedFiles && selectedFiles.length > 0) {
      await uploadFiles(Array.from(selectedFiles));
    }
  };

  const uploadFiles = async (filesToUpload: globalThis.File[]) => {
    for (const file of filesToUpload) {
      setUploadProgress(0);
      
      const formData = new FormData();
      formData.append("file", file);
      
      try {
        const response = await fetch("/api/files/upload", {
          method: "POST",
          body: formData,
        });
        
        if (response.ok) {
          const newFile = await response.json();
          setFiles((prev) => [newFile, ...prev]);
          setUploadProgress(100);
          
          // Refresh user data to get updated balance
          const userRes = await fetch("/api/user");
          if (userRes.ok) {
            setUser(await userRes.json());
          }
        } else {
          const error = await response.json();
          alert(error.error || "Upload failed");
        }
      } catch (error) {
        console.error("Upload error:", error);
        alert("Upload failed");
      } finally {
        setTimeout(() => setUploadProgress(null), 1000);
      }
    }
  };

  const handleDelete = async (fileId: string) => {
    if (!confirm("Are you sure you want to delete this file?")) return;
    
    try {
      const response = await fetch(`/api/files/${fileId}`, {
        method: "DELETE",
      });
      
      if (response.ok) {
        setFiles((prev) => prev.filter((f) => f.id !== fileId));
      }
    } catch (error) {
      console.error("Delete error:", error);
    }
  };

  const handleTopUp = async () => {
    if (!walletAddress) {
      alert("Please enter your wallet address");
      return;
    }
    
    try {
      const response = await fetch("/api/payments/topup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          amount: parseFloat(topUpAmount) * 100, // Convert to cents
          walletAddress,
        }),
      });
      
      if (response.ok) {
        const data = await response.json();
        setUser((prev) => prev ? { ...prev, balance: data.newBalance } : null);
        setShowTopUp(false);
        setTopUpAmount("1.00");
      } else {
        const error = await response.json();
        alert(error.error || "Top-up failed");
      }
    } catch (error) {
      console.error("Top-up error:", error);
    }
  };

  const totalStorage = files.reduce((acc, f) => acc + f.size, 0);
  const totalCostPerHour = files.reduce((acc, f) => acc + calculateCostPerHour(f.size), 0);

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-100">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-emerald-400 to-emerald-600 rounded-lg flex items-center justify-center">
              <Vault className="w-5 h-5 text-white" />
            </div>
            <span className="text-xl font-semibold text-gray-900">MicroVault</span>
          </Link>

          <div className="flex items-center gap-4">
            <button
              onClick={() => setShowTopUp(true)}
              className="flex items-center gap-2 px-4 py-2 bg-emerald-50 text-emerald-700 rounded-full hover:bg-emerald-100 transition-colors"
            >
              <Wallet className="w-4 h-4" />
              <span className="font-medium">{formatCurrency(user?.balance || 0)}</span>
              <Plus className="w-4 h-4" />
            </button>
            
            <div className="w-9 h-9 bg-gradient-to-br from-violet-400 to-purple-500 rounded-full flex items-center justify-center text-white text-sm font-medium">
              {user?.name?.[0] || "U"}
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8">
        {/* Stats Cards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
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
              <div className="w-12 h-12 bg-violet-50 rounded-xl flex items-center justify-center">
                <Clock className="w-6 h-6 text-violet-600" />
              </div>
              <div>
                <p className="text-sm text-gray-500">Cost per Hour</p>
                <p className="text-2xl font-semibold text-gray-900">{formatCurrency(totalCostPerHour * 100)}</p>
              </div>
            </div>
          </div>
          
          <div className="bg-white rounded-2xl p-6 border border-gray-100">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 bg-amber-50 rounded-xl flex items-center justify-center">
                <Wallet className="w-6 h-6 text-amber-600" />
              </div>
              <div>
                <p className="text-sm text-gray-500">Balance</p>
                <p className="text-2xl font-semibold text-gray-900">{formatCurrency(user?.balance || 0)}</p>
              </div>
            </div>
          </div>
        </div>

        {/* Upload Area */}
        <div
          className={`relative mb-8 border-2 border-dashed rounded-2xl p-12 text-center transition-all duration-200 ${
            isDragging
              ? "border-emerald-500 bg-emerald-50"
              : "border-gray-200 bg-white hover:border-gray-300"
          }`}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
        >
          <input
            type="file"
            multiple
            onChange={handleFileSelect}
            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
          />
          
          <div className="w-16 h-16 bg-gray-100 rounded-2xl flex items-center justify-center mx-auto mb-4">
            <Upload className={`w-8 h-8 ${isDragging ? "text-emerald-600" : "text-gray-400"}`} />
          </div>
          
          <p className="text-lg font-medium text-gray-900 mb-2">
            {isDragging ? "Drop files here" : "Drag and drop files here"}
          </p>
          <p className="text-gray-500">
            or click to browse • $0.001 per GB per hour
          </p>
          
          {uploadProgress !== null && (
            <div className="mt-6 max-w-xs mx-auto">
              <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
                <div 
                  className="h-full bg-emerald-500 transition-all duration-300"
                  style={{ width: `${uploadProgress}%` }}
                />
              </div>
              <p className="text-sm text-gray-500 mt-2">
                {uploadProgress === 100 ? "Upload complete!" : "Uploading..."}
              </p>
            </div>
          )}
        </div>

        {/* Files List */}
        <div className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-100">
            <h2 className="text-lg font-semibold text-gray-900">Your Files</h2>
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
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      File
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Type
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Size
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Cost/hr
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {files.map((file) => {
                    const FileIcon = getFileIcon(file.mimeType);
                    return (
                      <tr key={file.id} className="hover:bg-gray-50 transition-colors">
                        <td className="px-6 py-4">
                          <div className="flex items-center gap-3">
                            <div className="w-10 h-10 bg-gray-100 rounded-lg flex items-center justify-center">
                              <FileIcon className="w-5 h-5 text-gray-500" />
                            </div>
                            <span className="font-medium text-gray-900 truncate max-w-[200px]">
                              {file.name}
                            </span>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-gray-500">
                          {getFileTypeLabel(file.mimeType)}
                        </td>
                        <td className="px-6 py-4 text-gray-500">
                          {formatBytes(file.size)}
                        </td>
                        <td className="px-6 py-4">
                          <span className="text-emerald-600 font-medium">
                            {formatCurrency(calculateCostPerHour(file.size) * 100)}
                          </span>
                        </td>
                        <td className="px-6 py-4">
                          {file.isLocked ? (
                            <span className="inline-flex items-center gap-1 px-2 py-1 bg-red-50 text-red-600 text-xs font-medium rounded-full">
                              <AlertCircle className="w-3 h-3" /> Locked
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-1 px-2 py-1 bg-emerald-50 text-emerald-600 text-xs font-medium rounded-full">
                              <Check className="w-3 h-3" /> Active
                            </span>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <div className="flex items-center justify-end gap-2">
                            <a
                              href={`/api/files/${file.id}/download`}
                              className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
                              title="Download"
                            >
                              <Download className="w-4 h-4" />
                            </a>
                            <button
                              onClick={() => handleDelete(file.id)}
                              className="p-2 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
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
      </main>

      {/* Top-up Modal */}
      {showTopUp && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl max-w-md w-full p-6">
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-xl font-semibold text-gray-900">Top Up Balance</h3>
              <button
                onClick={() => setShowTopUp(false)}
                className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Your Wallet Address
                </label>
                <input
                  type="text"
                  value={walletAddress}
                  onChange={(e) => setWalletAddress(e.target.value)}
                  placeholder="$wallet.example.com/alice"
                  className="w-full px-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:border-transparent"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Enter your Open Payments wallet address
                </p>
              </div>
              
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Amount (USD)
                </label>
                <div className="relative">
                  <span className="absolute left-4 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                  <input
                    type="number"
                    min="0.01"
                    step="0.01"
                    value={topUpAmount}
                    onChange={(e) => setTopUpAmount(e.target.value)}
                    className="w-full pl-8 pr-4 py-3 border border-gray-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:border-transparent"
                  />
                </div>
              </div>
              
              <div className="flex gap-2">
                {["1.00", "5.00", "10.00", "25.00"].map((amount) => (
                  <button
                    key={amount}
                    onClick={() => setTopUpAmount(amount)}
                    className={`flex-1 py-2 rounded-lg text-sm font-medium transition-colors ${
                      topUpAmount === amount
                        ? "bg-emerald-500 text-white"
                        : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                    }`}
                  >
                    ${amount}
                  </button>
                ))}
              </div>
            </div>
            
            <div className="mt-6 pt-6 border-t border-gray-100">
              <button
                onClick={handleTopUp}
                className="w-full bg-gray-900 text-white py-3 rounded-xl font-medium hover:bg-gray-800 transition-colors"
              >
                Top Up with Open Payments
              </button>
              <p className="text-xs text-gray-500 text-center mt-3">
                Powered by Interledger Protocol
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

