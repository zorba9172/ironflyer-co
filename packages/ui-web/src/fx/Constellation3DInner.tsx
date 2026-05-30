import { useEffect, useRef } from 'react';
import * as THREE from 'three';

export type Constellation3DNode = {
  id: string;
  /** 0..1 relative size (drives sphere radius); defaults to 0.5 */
  value?: number;
  color?: string;
  /** optional fixed position on a -1..1 cube; auto-laid-out on a sphere if omitted */
  x?: number;
  y?: number;
  z?: number;
};

export type Constellation3DLink = { source: string; target: string; color?: string };

export type Constellation3DInnerProps = {
  nodes: Constellation3DNode[];
  links: Constellation3DLink[];
  colors: string[];
  height: number;
  rotate?: boolean;
};

// A data-bound 3D node constellation: glowing spheres connected by faint neon
// edges, wrapped in a slowly rotating wireframe halo. Nodes without explicit
// coordinates are distributed on a Fibonacci sphere so any graph reads cleanly.
// Mirrors orchestrator state (agent teams, gate graphs) as a living 3D object.
export default function Constellation3DInner({ nodes, links, colors, height, rotate = true }: Constellation3DInnerProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
    let w = el.clientWidth || 480;
    const h = height;
    const palette = colors.length ? colors : ['#6B5CFF', '#00D4FF', '#FF4FD8'];

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(48, w / h, 0.1, 100);
    camera.position.z = 4.6;

    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setSize(w, h);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    el.appendChild(renderer.domElement);

    const group = new THREE.Group();
    scene.add(group);
    scene.add(new THREE.AmbientLight(0xffffff, 0.7));
    const pl = new THREE.PointLight(new THREE.Color(palette[1] ?? palette[0]), 18, 30);
    pl.position.set(3, 4, 5);
    scene.add(pl);

    // Position lookup — explicit coords (scaled to ~radius 1.7) or Fibonacci sphere.
    const R = 1.7;
    const pos = new Map<string, THREE.Vector3>();
    const n = nodes.length || 1;
    nodes.forEach((node, i) => {
      if (node.x != null && node.y != null && node.z != null) {
        pos.set(node.id, new THREE.Vector3(node.x * R, node.y * R, node.z * R));
        return;
      }
      const phi = Math.acos(1 - (2 * (i + 0.5)) / n);
      const theta = Math.PI * (1 + Math.sqrt(5)) * i;
      pos.set(
        node.id,
        new THREE.Vector3(R * Math.sin(phi) * Math.cos(theta), R * Math.cos(phi), R * Math.sin(phi) * Math.sin(theta)),
      );
    });

    const disposables: { dispose(): void }[] = [];

    // Edges first so nodes render over them.
    links.forEach((link) => {
      const a = pos.get(link.source);
      const b = pos.get(link.target);
      if (!a || !b) return;
      const geo = new THREE.BufferGeometry().setFromPoints([a, b]);
      const mat = new THREE.LineBasicMaterial({
        color: new THREE.Color(link.color ?? palette[0]),
        transparent: true,
        opacity: 0.28,
      });
      group.add(new THREE.Line(geo, mat));
      disposables.push(geo, mat);
    });

    // Nodes as glowing spheres scaled by value.
    nodes.forEach((node, i) => {
      const p = pos.get(node.id);
      if (!p) return;
      const tone = new THREE.Color(node.color ?? palette[i % palette.length]);
      const r = 0.08 + (node.value ?? 0.5) * 0.16;
      const geo = new THREE.SphereGeometry(r, 20, 20);
      const mat = new THREE.MeshStandardMaterial({
        color: tone,
        emissive: tone.clone().multiplyScalar(0.6),
        metalness: 0.3,
        roughness: 0.35,
      });
      const mesh = new THREE.Mesh(geo, mat);
      mesh.position.copy(p);
      group.add(mesh);
      // Soft halo shell.
      const haloGeo = new THREE.SphereGeometry(r * 1.7, 16, 16);
      const haloMat = new THREE.MeshBasicMaterial({ color: tone, transparent: true, opacity: 0.12 });
      const halo = new THREE.Mesh(haloGeo, haloMat);
      halo.position.copy(p);
      group.add(halo);
      disposables.push(geo, mat, haloGeo, haloMat);
    });

    // Wireframe halo that contains the whole graph.
    const shellGeo = new THREE.IcosahedronGeometry(R * 1.32, 1);
    const shellMat = new THREE.MeshBasicMaterial({
      color: new THREE.Color(palette[palette.length - 1]),
      wireframe: true,
      transparent: true,
      opacity: 0.1,
    });
    const shell = new THREE.Mesh(shellGeo, shellMat);
    group.add(shell);
    disposables.push(shellGeo, shellMat);

    let raf = 0;
    const tick = () => {
      if (rotate && !reduceMotion) {
        group.rotation.y += 0.0026;
        group.rotation.x = Math.sin(group.rotation.y * 0.5) * 0.12;
      }
      renderer.render(scene, camera);
      raf = requestAnimationFrame(tick);
    };
    tick();

    const ro = new ResizeObserver(() => {
      w = el.clientWidth || w;
      renderer.setSize(w, h);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    });
    ro.observe(el);

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      disposables.forEach((d) => d.dispose());
      renderer.dispose();
      if (renderer.domElement.parentNode === el) el.removeChild(renderer.domElement);
    };
  }, [nodes, links, colors, height, rotate]);

  return <div ref={ref} style={{ width: '100%', height }} aria-hidden="true" />;
}
