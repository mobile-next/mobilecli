// Reads just enough of a png or jpeg to tell how many pixels it holds and
// whether it asks its viewer to rotate them, without pulling in an image library.

export type ImageSize = { width: number; height: number };

type JpegSegment = { marker: number; data: Buffer };

// what a viewer does when the orientation tag is absent
export const EXIF_UPRIGHT = 1;

const PNG_SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
const EXIF_ORIENTATION_TAG = 0x0112;

export function sizeOf(image: Buffer): ImageSize {
  return isPng(image) ? pngSize(image) : jpegSize(image);
}

// The orientation a viewer would apply before showing the image: 1 leaves the
// pixels as they are, anything else rotates or mirrors them.
export function exifOrientationOf(image: Buffer): number {
  const tiff = isPng(image) ? pngExifBlock(image) : jpegExifBlock(image);
  if (!tiff) {
    return EXIF_UPRIGHT;
  }
  return orientationInTiff(tiff) ?? EXIF_UPRIGHT;
}

function isPng(image: Buffer): boolean {
  return image.subarray(0, PNG_SIGNATURE.length).equals(PNG_SIGNATURE);
}

// the IHDR chunk always comes first, right after the signature
function pngSize(image: Buffer): ImageSize {
  return { width: image.readUInt32BE(16), height: image.readUInt32BE(20) };
}

function pngExifBlock(image: Buffer): Buffer | undefined {
  let offset = PNG_SIGNATURE.length;
  while (offset + 8 <= image.length) {
    const length = image.readUInt32BE(offset);
    const type = image.toString("latin1", offset + 4, offset + 8);
    if (type === "eXIf") {
      return image.subarray(offset + 8, offset + 8 + length);
    }
    // length + type + data + crc
    offset += 12 + length;
  }
  return undefined;
}

function jpegSize(image: Buffer): ImageSize {
  for (const segment of jpegSegments(image)) {
    if (isStartOfFrame(segment.marker)) {
      // precision (1 byte), then height and width
      return { width: segment.data.readUInt16BE(3), height: segment.data.readUInt16BE(1) };
    }
  }
  throw new Error("jpeg has no start-of-frame segment");
}

function jpegExifBlock(image: Buffer): Buffer | undefined {
  for (const segment of jpegSegments(image)) {
    if (segment.marker === 0xe1 && segment.data.toString("latin1", 0, 4) === "Exif") {
      // "Exif" is followed by two zero bytes, then the tiff block
      return segment.data.subarray(6);
    }
  }
  return undefined;
}

// walks the segments that precede the compressed image data
function jpegSegments(image: Buffer): JpegSegment[] {
  const segments: JpegSegment[] = [];
  let offset = 2; // past the start-of-image marker
  while (offset + 4 <= image.length && image[offset] === 0xff) {
    const marker = image[offset + 1];
    const length = image.readUInt16BE(offset + 2);
    segments.push({ marker, data: image.subarray(offset + 4, offset + 2 + length) });
    if (marker === 0xda) {
      break; // start of scan: pixels follow
    }
    offset += 2 + length;
  }
  return segments;
}

// SOF0..SOF15, minus the three markers in that range that are not frames
function isStartOfFrame(marker: number): boolean {
  return marker >= 0xc0 && marker <= 0xcf && ![0xc4, 0xc8, 0xcc].includes(marker);
}

function orientationInTiff(tiff: Buffer): number | undefined {
  const littleEndian = tiff.toString("latin1", 0, 2) === "II";
  const uint16 = (at: number) => (littleEndian ? tiff.readUInt16LE(at) : tiff.readUInt16BE(at));
  const uint32 = (at: number) => (littleEndian ? tiff.readUInt32LE(at) : tiff.readUInt32BE(at));

  const directory = uint32(4);
  const entries = uint16(directory);
  for (let index = 0; index < entries; index++) {
    const entry = directory + 2 + index * 12;
    if (uint16(entry) === EXIF_ORIENTATION_TAG) {
      return uint16(entry + 8);
    }
  }
  return undefined;
}
