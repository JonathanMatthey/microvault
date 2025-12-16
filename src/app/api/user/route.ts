import { NextRequest, NextResponse } from "next/server";
import { prisma } from "@/lib/prisma";
import { cookies } from "next/headers";

// Simple session management using cookies
async function getOrCreateUser() {
  const cookieStore = await cookies();
  let userId = cookieStore.get("userId")?.value;

  if (userId) {
    const user = await prisma.user.findUnique({
      where: { id: userId },
    });
    if (user) return user;
  }

  // Create a demo user for hackathon purposes
  const user = await prisma.user.create({
    data: {
      email: `demo-${Date.now()}@microvault.io`,
      name: "Demo User",
      balance: 100, // Start with $1.00 (100 cents) for testing
    },
  });

  return user;
}

export async function GET() {
  try {
    const user = await getOrCreateUser();
    
    // Set the user ID cookie
    const cookieStore = await cookies();
    cookieStore.set("userId", user.id, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "lax",
      maxAge: 60 * 60 * 24 * 30, // 30 days
    });

    return NextResponse.json({
      id: user.id,
      email: user.email,
      name: user.name,
      balance: user.balance,
      walletAddress: user.walletAddress,
    });
  } catch (error) {
    console.error("Error getting user:", error);
    return NextResponse.json(
      { error: "Failed to get user" },
      { status: 500 }
    );
  }
}

