import Image from '@tiptap/extension-image'

/**
 * 富文本里的视频节点。
 *
 * 它以 Image 为基类只是为了复用「原子块级媒体节点」这套结构，
 * 但**必须显式覆盖 addCommands 把继承来的 setImage 摘掉**——
 * tiptap 的 setImage 实现是 `insertContent({ type: this.name, ... })`，
 * 继承过来之后 this.name 会变成 'video'。而 Video 在 extensions 数组里
 * 排在 Image 之后，注册时后者覆盖前者，于是整个编辑器的 setImage 都会
 * 插入 video 节点——表现就是「上传的是图片，显示出来却是视频播放器」。
 */
export const VideoNode = Image.extend({
  name: 'video',
  group: 'block',
  inline: false,
  atom: true,
  addAttributes() {
    return {
      src: { default: null },
      controls: { default: true },
      preload: { default: 'metadata' },
    }
  },
  // 关键：显式覆盖 addCommands，把继承自 Image 的 setImage 摘掉。
  // 不摘的话它会带着 this.name='video' 一起注册，且 Video 在 extensions
  // 数组里排在 Image 之后，后注册者覆盖先注册者——整个编辑器的 setImage
  // 都会插入 video 节点。
  addCommands() {
    return {
      setVideo:
        (options: { src: string }) =>
        ({ commands }: { commands: { insertContent: (c: unknown) => boolean } }) =>
          commands.insertContent({ type: 'video', attrs: options }),
    } as never
  },
  parseHTML() {
    return [
      {
        tag: 'video',
        getAttrs: (element: HTMLElement) => ({
          src: element.getAttribute('src') || element.querySelector('source')?.getAttribute('src'),
          controls: true,
          preload: 'metadata',
        }),
      },
    ]
  },
  renderHTML({ HTMLAttributes }: { HTMLAttributes: Record<string, unknown> }) {
    return ['video', { ...HTMLAttributes, controls: 'controls', preload: 'metadata' }]
  },
})
