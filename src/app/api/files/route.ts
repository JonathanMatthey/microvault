import { NextRequest, NextResponse } from "next/server";
import { prisma } from "@/lib/prisma";
import { cookies } from "next/headers";

async function getUserId(): Promise<string | null> {
  const cookieStore = await cookies();
  return cookieStore.get("userId")?.value || null;
}

export async function GET() {
  try {
    const userId = await getUserId();
    
    if (!userId) {
      return NextResponse.json([]);
    }

    const files = await prisma.file.findMany({
      where: { userId },
      orderBy: { uploadedAt: "desc" },
    });

    return NextResponse.json(
      files.map((file) => ({
        id: file.id,
        name: file.name,
        size: file.size,
        mimeType: file.mimeType,
        uploadedAt: file.uploadedAt.toISOString(),
        totalCost: file.totalCost,
        isLocked: file.isLocked,
      }))
    );
  } catch (error) {
    console.error("Error listing files:", error);
    return NextResponse.json(
      { error: "Failed to list files" },
      { status: 500 }
    );
  }
}

