import { useEffect, useRef } from 'react';
import * as THREE from 'three';
import { palette } from '@ironflyer/design-tokens/brand';

// A quiet brand accent: a slowly rotating wireframe in cobalt + cyan.
export default function Scene3DInner({ height = 320 }: { height?: number }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const h = height;
    let w = el.clientWidth || 320;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(50, w / h, 0.1, 100);
    camera.position.z = 3.2;

    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setSize(w, h);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    el.appendChild(renderer.domElement);

    const core = new THREE.Mesh(
      new THREE.IcosahedronGeometry(1.15, 1),
      new THREE.MeshBasicMaterial({ color: new THREE.Color(palette.cobalt), wireframe: true }),
    );
    const halo = new THREE.Mesh(
      new THREE.IcosahedronGeometry(1.4, 1),
      new THREE.MeshBasicMaterial({ color: new THREE.Color(palette.cyan), wireframe: true, transparent: true, opacity: 0.22 }),
    );
    scene.add(core, halo);

    let raf = 0;
    const tick = () => {
      core.rotation.x += 0.0028;
      core.rotation.y += 0.0045;
      halo.rotation.x -= 0.0018;
      halo.rotation.y -= 0.0026;
      renderer.render(scene, camera);
      raf = requestAnimationFrame(tick);
    };
    tick();

    const onResize = () => {
      w = el.clientWidth || w;
      renderer.setSize(w, h);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    };
    window.addEventListener('resize', onResize);

    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', onResize);
      core.geometry.dispose();
      halo.geometry.dispose();
      (core.material as THREE.Material).dispose();
      (halo.material as THREE.Material).dispose();
      renderer.dispose();
      if (renderer.domElement.parentNode === el) el.removeChild(renderer.domElement);
    };
  }, [height]);

  return <div ref={ref} style={{ width: '100%', height }} aria-hidden="true" />;
}
