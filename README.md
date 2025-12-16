# MicroVault 🔐

**Pay-per-use file hosting powered by Open Payments**

Store files and pay only for what you use - per GB, per hour. No subscriptions, no minimums, just simple micropayments.

![MicroVault](https://img.shields.io/badge/ILP-Hackathon%202025-emerald)

## Features

- 📁 **Upload any file type** - Drag and drop or click to browse
- 💰 **Pay per GB per hour** - Simple pricing at $0.001/GB/hour
- ⚡ **Open Payments integration** - Top up balance using your ILP wallet
- 📊 **Real-time billing** - See costs as they accrue
- 🔒 **Secure storage** - Files locked when balance runs out
- 📥 **Easy downloads** - One-click file retrieval

## Tech Stack

- **Frontend**: Next.js 16, React, Tailwind CSS
- **Backend**: Next.js API Routes
- **Database**: SQLite with Prisma ORM
- **Payments**: Open Payments / Interledger Protocol
- **Icons**: Lucide React

## Getting Started

### Prerequisites

- Node.js 18+
- npm

### Installation

```bash
# Clone the repository
git clone <repo-url>
cd microvault

# Install dependencies
npm install

# Set up the database
npx prisma generate
npx prisma migrate dev

# Start the development server
npm run dev
```

### Environment Variables

Create a `.env` file:

```env
DATABASE_URL="file:./dev.db"
```

For production with Open Payments, you'll also need:

```env
WALLET_ADDRESS="https://wallet.example.com/your-wallet"
PRIVATE_KEY="your-private-key"
KEY_ID="your-key-id"
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/user` | GET | Get or create user |
| `/api/files` | GET | List user's files |
| `/api/files/upload` | POST | Upload a file |
| `/api/files/[id]` | DELETE | Delete a file |
| `/api/files/[id]/download` | GET | Download a file |
| `/api/payments/topup` | POST | Add balance via Open Payments |

## Pricing

| Storage | Cost |
|---------|------|
| 1 GB for 1 hour | $0.001 |
| 1 GB for 1 day | $0.024 |
| 1 GB for 1 month | ~$0.72 |

Files are billed per hour based on their size. Delete files anytime to stop charges.

## Open Payments Integration

MicroVault uses the [Open Payments](https://openpayments.dev) standard for receiving payments. Users can top up their balance using any Open Payments-enabled wallet.

### Payment Flow

1. User enters their wallet address
2. MicroVault creates an incoming payment request
3. User approves the payment in their wallet
4. Balance is credited to user's account
5. Files remain accessible while balance > 0

## Screenshots

### Landing Page
Clean, minimal design with emerald green accents and a product mockup.

### Dashboard
File management interface with:
- Storage stats
- Cost per hour tracking
- File list with actions
- Balance display and top-up

## License

MIT

## Built for ILP Hackathon 2025

This project was built as part of the Interledger Protocol Hackathon, demonstrating micropayments for file storage using Open Payments.

---

Made with 💚 using Open Payments
