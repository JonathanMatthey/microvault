"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import {
  Vault,
  Upload,
  Clock,
  Zap,
  ArrowRight,
  ChevronDown,
} from "lucide-react";

export default function Home() {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<{ email: string; id: string; credits: number } | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  // Load token & user on mount
  useEffect(() => {
    const stored = localStorage.getItem("googleIdToken");
    if (stored) {
      setToken(stored);
      // Fetch minimal user info to show identity banner
      const BACKEND_URL = process.env.NEXT_PUBLIC_BACKEND_URL || "http://localhost:8080";
      fetch(`${BACKEND_URL}/user`, {
        headers: { Authorization: `Bearer ${stored}` },
      })
        .then(async (res) => {
          if (res.status === 401 || res.status === 403) {
            localStorage.removeItem("googleIdToken");
            setToken(null);
            setUser(null);
            setStatus("Please sign in to continue.");
            return null;
          }
          if (!res.ok) return null;
          return res.json();
        })
        .then((data) => {
          if (data) setUser(data);
        })
        .catch(() => {});
    }
  }, []);

  const handleLogout = () => {
    localStorage.removeItem("googleIdToken");
    setToken(null);
    setUser(null);
    setStatus(null);
  };

  return (
    <div className="min-h-screen bg-white">
      {/* Header */}
      <header className="fixed top-0 left-0 right-0 bg-white/80 backdrop-blur-md z-50 border-b border-gray-100">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-emerald-400 to-emerald-600 rounded-lg flex items-center justify-center">
              <Vault className="w-5 h-5 text-white" />
            </div>
            <span className="text-xl font-semibold text-gray-900">
              SelfStack
            </span>
          </Link>

          <nav className="hidden md:flex items-center gap-8">
            <button className="flex items-center gap-1 text-gray-600 hover:text-gray-900 transition-colors">
              Product <ChevronDown className="w-4 h-4" />
            </button>
            <Link
              href="#pricing"
              className="text-gray-600 hover:text-gray-900 transition-colors"
            >
              Pricing
            </Link>
            <Link
              href="#about"
              className="text-gray-600 hover:text-gray-900 transition-colors"
            >
              About
            </Link>
            <Link
              href="#help"
              className="text-gray-600 hover:text-gray-900 transition-colors"
            >
              Help
            </Link>
          </nav>

          <div className="flex items-center gap-4">
            {token && user ? (
              <div className="flex items-center gap-3">
                <div className="text-right">
                  <p className="text-xs text-gray-500">Signed in as</p>
                  <p className="text-sm font-medium text-gray-900 truncate max-w-[240px]">{user.email || user.id}</p>
                </div>
                <button
                  onClick={handleLogout}
                  className="px-4 py-2 bg-gray-100 text-gray-700 rounded-full hover:bg-gray-200 transition-colors"
                >
                  Log out
                </button>
                <Link 
                  href="/dashboard" 
                  className="px-4 py-2 bg-gray-900 text-white rounded-full hover:bg-gray-800 transition-colors"
                >
                  Dashboard
                </Link>
              </div>
            ) : (
              <Link 
                href="/dashboard" 
                className="bg-gray-900 text-white px-5 py-2.5 rounded-full font-medium hover:bg-gray-800 transition-colors"
              >
                Log In
              </Link>
            )}
          </div>
        </div>
      </header>

      {/* Hero Section */}
      <main className="pt-24">
        <section className="max-w-7xl mx-auto px-6 py-16 md:py-24">
          <div className="grid lg:grid-cols-2 gap-12 items-center">
            {/* Left Column - Text */}
            <div className="opacity-0 animate-fade-in-up">
              {/* Announcement Badge */}
              <div className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-50 border border-emerald-200 rounded-full text-emerald-700 text-sm font-medium mb-8">
                <span>Full ownership, no suprises.</span>
                <span className="text-emerald-500">🔓</span>
              </div>

              <h1 className="font-serif text-5xl md:text-6xl lg:text-7xl font-semibold text-gray-900 leading-[1.1] mb-6">
              Self-hosted storage with
              <br />
               <span className="text-emerald-500">metered payments</span>
              </h1>

              <p className="text-lg md:text-xl text-gray-500 mb-10 max-w-lg leading-relaxed">
                Storage you actually control. No
                subscriptions, no minimums, just simple pay-as-you-go pricing.
              </p>

              <div className="flex flex-wrap items-center gap-4">
                <Link
                  href="/dashboard"
                  className="inline-flex items-center gap-2 bg-gray-900 text-white px-7 py-4 rounded-full font-medium hover:bg-gray-800 transition-all hover:gap-3"
                >
                  Get Started <ArrowRight className="w-5 h-5" />
                </Link>
                <Link
                  href="#how-it-works"
                  className="text-gray-600 hover:text-gray-900 font-medium underline underline-offset-4 decoration-gray-300 hover:decoration-gray-900 transition-colors"
                >
                  Learn More
                </Link>
              </div>
            </div>

            {/* Right Column - Hero Image */}
            <div className="relative opacity-0 animate-fade-in-up animation-delay-200 lg:scale-125 lg:translate-x-8">
              <Image
                src="/house.jpg"
                alt="SelfStack - Self-hosted storage"
                width={900}
                height={750}
                className="w-full h-auto"
                priority
              />
            </div>
          </div>
        </section>

        {/* Stats Section */}
        <section className="border-t border-gray-100">
          <div className="max-w-7xl mx-auto px-6 py-16">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
              {[
                {
                  value: "$0.001",
                  label: "per GB per hour",
                  sublabel: "Simple pricing",
                },
                {
                  value: "10GB",
                  label: "free storage",
                  sublabel: "To get started",
                },
                {
                  value: "99.9%",
                  label: "uptime",
                  sublabel: "Reliable hosting",
                },
                { value: "∞", label: "file types", sublabel: "Store anything" },
              ].map((stat, i) => (
                <div
                  key={i}
                  className={`text-center opacity-0 animate-fade-in-up`}
                  style={{
                    animationDelay: `${(i + 1) * 100}ms`,
                    animationFillMode: "forwards",
                  }}
                >
                  <p className="text-3xl md:text-4xl font-semibold text-gray-900 mb-1">
                    {stat.value}
                  </p>
                  <p className="text-gray-500">{stat.label}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* How it Works Section */}
        <section
          id="how-it-works"
          className="bg-gray-50 border-t border-gray-100"
        >
          <div className="max-w-7xl mx-auto px-6 py-24">
            <div className="text-center mb-16">
              <h2 className="font-serif text-4xl md:text-5xl font-semibold text-gray-900 mb-4">
                How it works
              </h2>
              <p className="text-lg text-gray-500 max-w-2xl mx-auto">
                Simple, transparent pricing powered by Open Payments. No credit
                cards, no hidden fees.
              </p>
            </div>

            <div className="grid md:grid-cols-3 gap-8">
              {[
                {
                  icon: Upload,
                  title: "Upload your files",
                  description:
                    "Drag and drop any file type. We'll start the meter as soon as your upload completes.",
                },
                {
                  icon: Clock,
                  title: "Pay per hour",
                  description:
                    "Your files are billed at $0.001 per GB per hour. Delete anytime to stop charges.",
                },
                {
                  icon: Zap,
                  title: "Instant payments",
                  description:
                    "Top up your balance using your Open Payments wallet. Micropayments made simple.",
                },
              ].map((step, i) => (
                <div
                  key={i}
                  className="bg-white rounded-2xl p-8 border border-gray-100 hover:border-emerald-200 hover:shadow-lg transition-all duration-300"
                >
                  <div className="w-14 h-14 bg-emerald-50 rounded-xl flex items-center justify-center mb-6">
                    <step.icon className="w-7 h-7 text-emerald-600" />
                  </div>
                  <h3 className="text-xl font-semibold text-gray-900 mb-3">
                    {step.title}
                  </h3>
                  <p className="text-gray-500 leading-relaxed">
                    {step.description}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* CTA Section */}
        <section className="bg-gray-900">
          <div className="max-w-7xl mx-auto px-6 py-24 text-center">
            <h2 className="font-serif text-4xl md:text-5xl font-semibold text-white mb-6">
              Ready to start storing?
            </h2>
            <p className="text-lg text-gray-400 mb-10 max-w-xl mx-auto">
              No sign-up required. Just connect your wallet and start uploading.
            </p>
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-2 bg-emerald-500 text-white px-8 py-4 rounded-full font-medium hover:bg-emerald-400 transition-all hover:gap-3"
            >
              Launch App <ArrowRight className="w-5 h-5" />
            </Link>
          </div>
        </section>

        {/* Footer */}
        <footer className="border-t border-gray-100 bg-white">
          <div className="max-w-7xl mx-auto px-6 py-12">
            <div className="flex flex-col md:flex-row items-center justify-between gap-6">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 bg-gradient-to-br from-emerald-400 to-emerald-600 rounded-lg flex items-center justify-center">
                  <Vault className="w-5 h-5 text-white" />
                </div>
                <span className="text-xl font-semibold text-gray-900">
                  SelfStack
                </span>
              </div>

              <p className="text-gray-500 text-sm">
                Built with Open Payments • ILP Hackathon 2025
              </p>

              <div className="flex items-center gap-6 text-sm text-gray-500">
                <Link
                  href="#"
                  className="hover:text-gray-900 transition-colors"
                >
                  Privacy
                </Link>
                <Link
                  href="#"
                  className="hover:text-gray-900 transition-colors"
                >
                  Terms
                </Link>
                <Link
                  href="#"
                  className="hover:text-gray-900 transition-colors"
                >
                  GitHub
                </Link>
              </div>
            </div>
          </div>
        </footer>
      </main>
    </div>
  );
}
