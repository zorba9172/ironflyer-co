import { Box } from "@mui/material";
import { tokens } from "../../theme";

export function BrandBackdrop() {
  return (
    <Box
      aria-hidden
      sx={{
        position: "absolute",
        inset: 0,
        overflow: "hidden",
        pointerEvents: "none",
        "&::before": {
          content: '""',
          position: "absolute",
          inset: "-2% -6% auto 34%",
          height: { xs: 420, md: 660 },
          backgroundImage: "url('/market/data-flow.jpg')",
          backgroundSize: "cover",
          backgroundPosition: "center right",
          opacity: 0.34,
          filter: "saturate(1.22) contrast(1.08)",
          mixBlendMode: "screen",
          transform: "scale(1.04)",
          animation: "ironflyerMediaDrift 20s ease-in-out infinite alternate",
        },
        "&::after": {
          content: '""',
          position: "absolute",
          inset: 0,
          background: [
            `linear-gradient(90deg, ${tokens.color.bg.base} 0%, ${tokens.color.bg.base}f5 34%, ${tokens.color.bg.base}b8 58%, ${tokens.color.bg.base}f7 100%)`,
            `linear-gradient(180deg, ${tokens.color.bg.base}1f 0%, ${tokens.color.bg.base}e6 82%, ${tokens.color.bg.base} 100%)`,
            `radial-gradient(ellipse 900px 420px at 18% 18%, ${tokens.color.accent.coral}18, ${tokens.color.bg.base}00 70%)`,
            `radial-gradient(ellipse 720px 460px at 92% 16%, ${tokens.color.accent.violet}28, ${tokens.color.bg.base}00 72%)`,
            `radial-gradient(circle at 1px 1px, ${tokens.color.text.primary}18 1px, ${tokens.color.bg.base}00 1.5px)`,
          ].join(", "),
          backgroundSize: "auto, auto, auto, auto, 30px 30px",
        },
        ".ifly-orbital": {
          position: "absolute",
          right: { xs: -120, md: 44 },
          top: { xs: 22, md: 42 },
          width: { xs: 210, md: 300 },
          aspectRatio: "1 / 1",
          borderRadius: "50%",
          border: `1px solid ${tokens.color.accent.violet}42`,
          background: [
            `radial-gradient(circle at 34% 28%, ${tokens.color.text.primary}3d, ${tokens.color.accent.violet}2e 24%, ${tokens.color.bg.surfaceRaised}f0 48%, ${tokens.color.bg.base} 72%)`,
            `linear-gradient(135deg, ${tokens.color.accent.violet}66, ${tokens.color.bg.base}00 52%)`,
          ].join(", "),
          boxShadow: `0 0 70px ${tokens.color.accent.violet}3b, inset -28px -28px 58px ${tokens.color.bg.base}`,
          opacity: 0.9,
          transform: "rotate(-18deg)",
          animation: "ironflyerOrbitalFloat 12s ease-in-out infinite alternate",
        },
        ".ifly-orbital::after": {
          content: '""',
          position: "absolute",
          left: "-18%",
          top: "44%",
          width: "136%",
          height: "22%",
          borderTop: `2px solid ${tokens.color.accent.violet}66`,
          borderRadius: "50%",
          transform: "rotate(-18deg)",
          filter: "blur(0.2px)",
        },
        ".ifly-poly": {
          position: "absolute",
          left: { xs: "70%", md: "42%" },
          top: { xs: 260, md: 250 },
          width: { xs: 54, md: 82 },
          aspectRatio: "1 / 1",
          borderRadius: 1,
          transform: "rotateX(62deg) rotateZ(45deg)",
          background: `linear-gradient(135deg, ${tokens.color.accent.violet}55, ${tokens.color.bg.surfaceRaised}11)`,
          border: `1px solid ${tokens.color.accent.violet}66`,
          boxShadow: `0 0 34px ${tokens.color.accent.violet}52, inset 0 0 26px ${tokens.color.accent.violet}33`,
          animation: "ironflyerPolySpin 9s ease-in-out infinite alternate",
        },
        "@keyframes ironflyerMediaDrift": {
          "0%": { transform: "scale(1.03) translate3d(-1%, -1%, 0)" },
          "100%": { transform: "scale(1.09) translate3d(1.5%, 1%, 0)" },
        },
        "@keyframes ironflyerOrbitalFloat": {
          "0%": { transform: "translate3d(0, 0, 0) rotate(-18deg)" },
          "100%": { transform: "translate3d(-18px, 20px, 0) rotate(-10deg)" },
        },
        "@keyframes ironflyerPolySpin": {
          "0%": { transform: "rotateX(62deg) rotateZ(30deg) translate3d(0, 0, 0)" },
          "100%": { transform: "rotateX(52deg) rotateZ(82deg) translate3d(8px, -12px, 0)" },
        },
      }}
    >
      <Box className="ifly-orbital" />
      <Box className="ifly-poly" />
    </Box>
  );
}
