import { useEffect, useRef } from 'react';
import * as THREE from 'three';

export type Bar3DDatum = { label: string; value: number; color?: string };

export type Bars3DInnerProps = {
  data: Bar3DDatum[];
  colors: string[];
  height: number;
  /** value mapped to the tallest bar; defaults to the data max */
  max?: number;
  rotate?: boolean;
};

// A data-bound field of 3D bars on a reflective neon grid. Each bar's height is
// proportional to its value; colors cycle through the provided palette (or the
// datum's own color). Pure three.js, mounted client-side behind the lazy
// boundary in Bars3D. Disposes every GPU resource on unmount.
export default function Bars3DInner({ data, colors, height, max, rotate = true }: Bars3DInnerProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
    let w = el.clientWidth || 480;
    const h = height;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(42, w / h, 0.1, 100);
    camera.position.set(0, 5.4, 9.2);
    camera.lookAt(0, 0.6, 0);

    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setSize(w, h);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    el.appendChild(renderer.domElement);

    const group = new THREE.Group();
    scene.add(group);

    // Neon point lights for depth + a soft ambient fill.
    scene.add(new THREE.AmbientLight(0xffffff, 0.55));
    const palette = colors.length ? colors : ['#6B5CFF', '#00D4FF', '#FF4FD8'];
    const l1 = new THREE.PointLight(new THREE.Color(palette[0]), 26, 40);
    l1.position.set(-5, 7, 6);
    const l2 = new THREE.PointLight(new THREE.Color(palette[Math.min(2, palette.length - 1)]), 22, 40);
    l2.position.set(6, 5, 4);
    scene.add(l1, l2);

    const n = Math.max(data.length, 1);
    const span = 8;
    const slot = span / n;
    const barW = Math.min(slot * 0.56, 0.92);
    const peak = max ?? Math.max(...data.map((d) => d.value), 1);
    const maxH = 4.4;

    const meshes: THREE.Mesh[] = [];
    data.forEach((d, i) => {
      const tone = new THREE.Color(d.color ?? palette[i % palette.length]);
      const barH = Math.max((d.value / (peak || 1)) * maxH, 0.06);
      const geo = new THREE.BoxGeometry(barW, barH, barW);
      const mat = new THREE.MeshStandardMaterial({
        color: tone,
        emissive: tone.clone().multiplyScalar(0.35),
        metalness: 0.4,
        roughness: 0.28,
        transparent: true,
        opacity: 0.96,
      });
      const mesh = new THREE.Mesh(geo, mat);
      mesh.position.set(-span / 2 + slot / 2 + i * slot, barH / 2, 0);
      group.add(mesh);
      meshes.push(mesh);
    });

    // Reflective base grid.
    const grid = new THREE.GridHelper(span + 2, 16, new THREE.Color(palette[0]), new THREE.Color(palette[0]));
    (grid.material as THREE.Material).opacity = 0.16;
    (grid.material as THREE.Material).transparent = true;
    group.add(grid);

    let raf = 0;
    let t = 0;
    const tick = () => {
      t += 0.01;
      if (rotate && !reduceMotion) group.rotation.y = Math.sin(t * 0.5) * 0.34;
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
      meshes.forEach((m) => {
        m.geometry.dispose();
        (m.material as THREE.Material).dispose();
      });
      grid.geometry.dispose();
      (grid.material as THREE.Material).dispose();
      renderer.dispose();
      if (renderer.domElement.parentNode === el) el.removeChild(renderer.domElement);
    };
  }, [data, colors, height, max, rotate]);

  return <div ref={ref} style={{ width: '100%', height }} aria-hidden="true" />;
}
