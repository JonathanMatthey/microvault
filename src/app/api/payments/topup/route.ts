import { NextRequest, NextResponse } from "next/server";
import { prisma } from "@/lib/prisma";
import { cookies } from "next/headers";

async function getUserId(): Promise<string | null> {
  const cookieStore = await cookies();
  return cookieStore.get("userId")?.value || null;
}

export async function POST(request: NextRequest) {
  try {
    const userId = await getUserId();
    
    if (!userId) {
      return NextResponse.json(
        { error: "Not authenticated" },
        { status: 401 }
      );
    }

    const body = await request.json();
    const { amount, walletAddress } = body;

    if (!amount || amount <= 0) {
      return NextResponse.json(
        { error: "Invalid amount" },
        { status: 400 }
      );
    }

    if (!walletAddress) {
      return NextResponse.json(
        { error: "Wallet address required" },
        { status: 400 }
      );
    }

    // For the hackathon demo, we'll simulate the payment
    // In production, this would:
    // 1. Create an incoming payment on our wallet
    // 2. Return the payment details for the user to complete
    // 3. Use webhooks or polling to confirm payment
    // 4. Credit the user's balance once confirmed

    // Simulate payment processing
    // In a real implementation, you would use the Open Payments SDK:
    // 
    // import { createAuthenticatedClient } from "@interledger/open-payments";
    // 
    // const client = await createAuthenticatedClient({
    //   walletAddressUrl: process.env.WALLET_ADDRESS,
    //   privateKey: process.env.PRIVATE_KEY,
    //   keyId: process.env.KEY_ID,
    // });
    // 
    // const walletAddress = await client.walletAddress.get({
    //   url: walletAddressUrl,
    // });
    // 
    // const incomingPayment = await client.incomingPayment.create(
    //   {
    //     url: walletAddress.resourceServer,
    //     accessToken: grant.access_token.value,
    //   },
    //   {
    //     walletAddress: walletAddressUrl,
    //     incomingAmount: {
    //       value: amount.toString(),
    //       assetCode: "USD",
    //       assetScale: 2,
    //     },
    //   }
    // );

    // For demo purposes, instantly credit the balance
    const user = await prisma.user.update({
      where: { id: userId },
      data: {
        balance: { increment: amount },
        walletAddress: walletAddress,
      },
    });

    // Record the transaction
    await prisma.transaction.create({
      data: {
        userId,
        amount,
        type: "deposit",
        description: `Top-up from ${walletAddress}`,
      },
    });

    // Unlock any locked files
    await prisma.file.updateMany({
      where: {
        userId,
        isLocked: true,
      },
      data: {
        isLocked: false,
      },
    });

    return NextResponse.json({
      success: true,
      newBalance: user.balance,
      message: "Balance topped up successfully",
    });
  } catch (error) {
    console.error("Error processing top-up:", error);
    return NextResponse.json(
      { error: "Failed to process top-up" },
      { status: 500 }
    );
  }
}

