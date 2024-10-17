
import styles from './styles.module.css';

export default function VideoIframe({ url }) {
  return (
    <div className={styles.videoIframe}>
      <iframe src={url} title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen>
      </iframe>
    </div>

  );
}