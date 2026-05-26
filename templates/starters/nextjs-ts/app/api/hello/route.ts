import { NextResponse } from "next/server";

export const runtime = "edge";

export function GET() {
  return NextResponse.json({
    message: "hello from {{PROJECT_NAME}}",
    scaffold: "ironflyer",
    at: "{{TODAY}}",
  });
}
